import { Link, useParams, useSearchParams } from 'react-router-dom'
import { api } from '../api/client'
import { usePolling } from '../hooks/usePolling'
import { formatINR, formatPct } from '../lib/format'
import { HoldingsPieChart } from '../components/HoldingsPieChart'
import { MetricCards } from '../components/MetricCards'
import { Panel } from '../components/Panel'
import { StatusBadge } from '../components/StatusBadge'
import './pages.css'

export function AgentDetailPage() {
  const { id: agentId = '' } = useParams()
  const [params] = useSearchParams()
  const simId = params.get('sim') ?? ''

  const portfolioQuery = usePolling(
    () => api.getAgentPortfolio(simId, agentId),
    5000,
    !!simId && !!agentId,
  )
  const metricsQuery = usePolling(
    () => api.getAgentMetrics(simId, agentId),
    5000,
    !!simId && !!agentId,
  )
  const ordersQuery = usePolling(
    () => api.getAgentOrders(simId, agentId, 50),
    5000,
    !!simId && !!agentId,
  )

  const portfolio = portfolioQuery.data
  const metrics = metricsQuery.data

  if (!simId) {
    return (
      <div className="page">
        <p className="page-error">Missing simulation context. Open an agent from a simulation leaderboard.</p>
        <Link to="/simulations">Back to simulations</Link>
      </div>
    )
  }

  return (
    <div className="page">
      <header className="page-header">
        <div>
          <Link to={`/simulation/${simId}`} className="back-link">
            ← Live simulation
          </Link>
          <h1>{metrics?.agent_name ?? portfolio?.agent_id.slice(0, 8) ?? 'Agent'}</h1>
          <p className="lede">Deep dive · as of {portfolio?.as_of_date ?? '—'}</p>
        </div>
        {portfolio ? (
          <div className="hero-stat">
            <span className="hero-stat-label">Portfolio value</span>
            <span className="hero-stat-value">{formatINR(portfolio.total_value)}</span>
            <span className={`hero-stat-delta ${portfolio.total_return_pct >= 0 ? 'profit' : 'loss'}`}>
              {formatPct(portfolio.total_return_pct)}
            </span>
          </div>
        ) : null}
      </header>

      {metrics ? (
        <Panel title="Performance metrics" subtitle="Risk-adjusted stats from EOD snapshots">
          <MetricCards metrics={metrics} />
        </Panel>
      ) : null}

      <div className="bento">
        <Panel title="Holdings" subtitle="Allocation by position value">
          {portfolio ? (
            <HoldingsPieChart holdings={portfolio.holdings} cash={portfolio.cash} />
          ) : (
            <p className="empty-state">Loading portfolio…</p>
          )}
        </Panel>

        <Panel title="Trade history" subtitle="Recent orders for this agent">
          {!ordersQuery.data?.length ? (
            <p className="empty-state">No trades yet.</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Time</th>
                    <th>Symbol</th>
                    <th>Side</th>
                    <th>Qty</th>
                    <th>Status</th>
                  </tr>
                </thead>
                <tbody>
                  {ordersQuery.data.map((o) => (
                    <tr key={o.id}>
                      <td>{new Date(o.created_at).toLocaleString('en-IN')}</td>
                      <td>{o.symbol}</td>
                      <td className={o.side === 'buy' ? 'profit' : 'loss'}>{o.side}</td>
                      <td className="num">{o.quantity}</td>
                      <td>
                        <StatusBadge status={o.status} />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </Panel>
      </div>
    </div>
  )
}
