import { useState } from 'react'
import { Link } from 'react-router-dom'
import { Plus, RefreshCw } from 'lucide-react'
import { api } from '../api/client'
import { usePolling } from '../hooks/usePolling'
import { StatusBadge } from '../components/StatusBadge'
import { Panel } from '../components/Panel'
import { CreateSimulationPanel } from '../components/CreateSimulationPanel'
import './pages.css'

export function SimulationsPage() {
  const { data, error, loading, refresh } = usePolling(
    () => api.listSimulations().then((r) => r.simulations),
    8000,
  )

  const [showForm, setShowForm] = useState(false)

  return (
    <div className="page">
      <header className="page-header">
        <div>
          <p className="eyebrow">Phase 5 · Dashboard</p>
          <h1>Simulations</h1>
          <p className="lede">
            Create runs with LLM agents, custom prompts, and one-click launch via the orchestrator.
          </p>
        </div>
        <div className="page-actions">
          <button type="button" className="btn ghost" onClick={() => refresh()} aria-label="Refresh">
            <RefreshCw size={16} />
          </button>
          <button type="button" className="btn primary" onClick={() => setShowForm((v) => !v)}>
            <Plus size={16} />
            New simulation
          </button>
        </div>
      </header>

      {showForm ? (
        <CreateSimulationPanel
          onCancel={() => setShowForm(false)}
          onCreated={() => {
            setShowForm(false)
            refresh()
          }}
        />
      ) : null}

      <Panel title="All simulations" subtitle={loading ? 'Loading…' : `${data?.length ?? 0} runs`}>
        {error ? <p className="page-error">{error}</p> : null}
        {!loading && data?.length === 0 ? (
          <p className="empty-state">No simulations yet. Create one to get started.</p>
        ) : null}
        <div className="sim-grid">
          {data?.map((sim) => (
            <Link key={sim.id} to={`/simulation/${sim.id}`} className="sim-card">
              <div className="sim-card-top">
                <h3>{sim.name}</h3>
                <StatusBadge status={sim.status} />
              </div>
              <dl className="sim-meta">
                <div>
                  <dt>Window</dt>
                  <dd>
                    {sim.start_date} → {sim.end_date}
                  </dd>
                </div>
                <div>
                  <dt>As of</dt>
                  <dd>{sim.as_of_date ?? 'Not started'}</dd>
                </div>
              </dl>
              <span className="sim-id">{sim.id.slice(0, 8)}…</span>
            </Link>
          ))}
        </div>
      </Panel>
    </div>
  )
}
