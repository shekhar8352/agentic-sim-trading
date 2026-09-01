import { useCallback, useMemo } from 'react'
import { Link, useParams } from 'react-router-dom'
import { ArrowRight, Pause, Play, Users } from 'lucide-react'
import { api } from '../api/client'
import { usePolling } from '../hooks/usePolling'
import { AgentPromptsEditor } from '../components/AgentPromptsEditor'
import { EquityChart } from '../components/EquityChart'
import { LeaderboardTable } from '../components/LeaderboardTable'
import { OrderFeed } from '../components/OrderFeed'
import { Panel } from '../components/Panel'
import { StatusBadge } from '../components/StatusBadge'
import './pages.css'

export function SimulationLivePage() {
  const { id = '' } = useParams()

  const simQuery = usePolling(() => api.getSimulation(id), 4000, !!id)
  const lbQuery = usePolling(() => api.getLeaderboard(id).then((r) => r.entries), 4000, !!id)
  const curvesQuery = usePolling(() => api.getEquityCurves(id).then((r) => r.curves), 4000, !!id)
  const ordersQuery = usePolling(() => api.getRecentOrders(id, 20).then((r) => r.orders), 4000, !!id)

  const sim = simQuery.data
  const awaitingCheckpoint = sim?.awaiting_proceed === true
  const status = awaitingCheckpoint
    ? 'checkpoint'
    : (sim?.clock_status ?? sim?.db_status ?? 'paused')
  const checkpointDays = sim?.checkpoint_interval_days ?? 5
  const orchestratorAgents = sim?.config?.orchestrator_agents ?? []

  const progress = useMemo(() => {
    if (!sim?.total_trading_days || sim.trading_day_index == null) return null
    return Math.round((sim.trading_day_index / sim.total_trading_days) * 100)
  }, [sim])

  const handleStart = useCallback(async () => {
    await api.startSimulation(id)
    await simQuery.refresh()
  }, [id, simQuery])

  const handlePause = useCallback(async () => {
    await api.pauseSimulation(id)
    await simQuery.refresh()
  }, [id, simQuery])

  const handleProceed = useCallback(async () => {
    await api.proceedSimulation(id)
    await simQuery.refresh()
  }, [id, simQuery])

  if (!id) return null

  return (
    <div className="page">
      <header className="page-header">
        <div>
          <Link to="/simulations" className="back-link">
            ← Simulations
          </Link>
          <h1>{sim?.name ?? 'Simulation'}</h1>
          <div className="live-meta">
            <StatusBadge status={status} />
            {sim?.bar_ts ? (
              <span className="tick-label">Tick: {sim.current_trading_day} {sim.bar_ts}</span>
            ) : sim?.current_trading_day ? (
              <span className="tick-label">Tick: {sim.current_trading_day}</span>
            ) : null}
            {progress != null ? (
              <span className="tick-label">{progress}% complete</span>
            ) : null}
          </div>
        </div>
        <div className="page-actions">
          {awaitingCheckpoint ? (
            <button type="button" className="btn primary" onClick={handleProceed}>
              <ArrowRight size={16} />
              Continue {checkpointDays} more days
            </button>
          ) : status === 'running' ? (
            <button type="button" className="btn ghost" onClick={handlePause}>
              <Pause size={16} />
              Pause
            </button>
          ) : (
            <button type="button" className="btn primary" onClick={handleStart}>
              <Play size={16} />
              Start
            </button>
          )}
          <Link to={`/compare?sim=${id}`} className="btn ghost">
            <Users size={16} />
            Compare agents
          </Link>
        </div>
      </header>

      {awaitingCheckpoint ? (
        <div className="checkpoint-banner" role="status">
          <div>
            <strong>Checkpoint reached</strong>
            <p>
              The simulation advanced {sim?.days_since_checkpoint ?? checkpointDays} trading days
              and paused. A full multi-year run can take a long time — continue for another{' '}
              {checkpointDays} days when you are ready.
            </p>
          </div>
          <button type="button" className="btn primary" onClick={handleProceed}>
            <ArrowRight size={16} />
            Continue
          </button>
        </div>
      ) : null}

      {progress != null ? (
        <div className="progress-bar" role="progressbar" aria-valuenow={progress} aria-valuemin={0} aria-valuemax={100}>
          <div className="progress-fill" style={{ width: `${progress}%` }} />
        </div>
      ) : null}

      {orchestratorAgents.length > 0 ? (
        <AgentPromptsEditor
          simulationId={id}
          agents={orchestratorAgents}
          onSaved={() => simQuery.refresh()}
        />
      ) : null}

      <div className="bento">
        <Panel
          className="bento-wide"
          title="Leaderboard"
          subtitle="Ranked by portfolio value"
        >
          <LeaderboardTable entries={lbQuery.data ?? []} simId={id} />
        </Panel>

        <Panel
          className="bento-wide"
          title="Equity curves"
          subtitle="Portfolio value over simulation time"
        >
          <EquityChart curves={curvesQuery.data ?? []} />
        </Panel>

        <Panel title="Live order feed" subtitle="Last 20 orders across agents">
          <OrderFeed orders={ordersQuery.data ?? []} simId={id} />
        </Panel>
      </div>
    </div>
  )
}
