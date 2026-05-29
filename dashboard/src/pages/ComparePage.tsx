import { useMemo, useState } from 'react'
import { Link, useSearchParams } from 'react-router-dom'
import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { api } from '../api/client'
import { usePolling } from '../hooks/usePolling'
import { formatINR, formatPct } from '../lib/format'
import { Panel } from '../components/Panel'
import './pages.css'

export function ComparePage() {
  const [params] = useSearchParams()
  const initialSim = params.get('sim') ?? ''

  const simsQuery = usePolling(() => api.listSimulations().then((r) => r.simulations), 15000)
  const [simId, setSimId] = useState(initialSim)

  const activeSim = simId || simsQuery.data?.[0]?.id || ''

  const lbQuery = usePolling(
    () => api.getLeaderboard(activeSim).then((r) => r.entries),
    5000,
    !!activeSim,
  )

  const chartData = useMemo(
    () =>
      (lbQuery.data ?? []).map((e) => ({
        name: e.agent_name || e.agent_id.slice(0, 8),
        return: e.total_return_pct,
        sharpe: e.sharpe_ratio,
        value: e.total_value,
        agentId: e.agent_id,
      })),
    [lbQuery.data],
  )

  return (
    <div className="page">
      <header className="page-header">
        <div>
          <p className="eyebrow">Head-to-head</p>
          <h1>Compare agents</h1>
          <p className="lede">Side-by-side performance across a simulation.</p>
        </div>
        <label className="sim-select">
          Simulation
          <select value={activeSim} onChange={(e) => setSimId(e.target.value)}>
            {(simsQuery.data ?? []).map((s) => (
              <option key={s.id} value={s.id}>
                {s.name}
              </option>
            ))}
          </select>
        </label>
      </header>

      {!activeSim ? (
        <p className="empty-state">Create a simulation first.</p>
      ) : (
        <>
          <Panel title="Return comparison" subtitle="Total return % by agent">
            <div className="compare-chart">
              <ResponsiveContainer width="100%" height={280}>
                <BarChart data={chartData} margin={{ top: 8, right: 8, left: 0, bottom: 0 }}>
                  <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
                  <XAxis
                    dataKey="name"
                    tick={{ fill: 'var(--text-dim)', fontSize: 10, fontFamily: 'var(--font-mono)' }}
                    tickLine={false}
                    axisLine={{ stroke: 'var(--border)' }}
                  />
                  <YAxis
                    tick={{ fill: 'var(--text-dim)', fontSize: 10, fontFamily: 'var(--font-mono)' }}
                    tickLine={false}
                    axisLine={false}
                    tickFormatter={(v) => `${v}%`}
                  />
                  <Tooltip
                    formatter={(value, name) =>
                      name === 'return' ? formatPct(Number(value)) : Number(value).toFixed(2)
                    }
                    contentStyle={{
                      background: 'var(--bg-elevated)',
                      border: '1px solid var(--border-strong)',
                      borderRadius: 'var(--radius-sm)',
                      fontFamily: 'var(--font-mono)',
                      fontSize: 12,
                    }}
                  />
                  <Bar dataKey="return" fill="var(--accent)" radius={[4, 4, 0, 0]} />
                </BarChart>
              </ResponsiveContainer>
            </div>
          </Panel>

          <Panel title="Agent cards" subtitle="Tap through to individual deep-dives">
            <div className="compare-grid">
              {chartData.map((a) => (
                <Link
                  key={a.agentId}
                  to={`/agent/${a.agentId}?sim=${activeSim}`}
                  className="compare-card"
                >
                  <h3>{a.name}</h3>
                  <p className={`compare-return ${a.return >= 0 ? 'profit' : 'loss'}`}>
                    {formatPct(a.return)}
                  </p>
                  <dl>
                    <div>
                      <dt>Value</dt>
                      <dd>{formatINR(a.value)}</dd>
                    </div>
                    <div>
                      <dt>Sharpe</dt>
                      <dd>{a.sharpe.toFixed(2)}</dd>
                    </div>
                  </dl>
                </Link>
              ))}
            </div>
          </Panel>
        </>
      )}
    </div>
  )
}
