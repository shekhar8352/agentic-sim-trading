import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Trash2 } from 'lucide-react'
import { orchestratorApi } from '../api/orchestratorClient'
import type { PersonalityInfo, ProviderInfo, StrategyInfo } from '../api/types'
import { Panel } from './Panel'
import '../pages/pages.css'

interface AgentDraft {
  key: string
  name: string
  provider: string
  model: string
  personality: string
  system_prompt: string
  prompt_customized: boolean
}

function newAgentDraft(
  defaultPersonality: string,
  defaultPrompt: string,
  defaultProvider?: ProviderInfo,
): AgentDraft {
  const provider = defaultProvider?.id ?? 'custom'
  const model = defaultProvider?.models[0] ?? 'momentum-v1'
  return {
    key: crypto.randomUUID(),
    name: '',
    provider,
    model,
    personality: defaultPersonality,
    system_prompt: defaultPrompt,
    prompt_customized: false,
  }
}

interface CreateSimulationPanelProps {
  onCancel: () => void
  onCreated: () => void
}

export function CreateSimulationPanel({ onCancel, onCreated }: CreateSimulationPanelProps) {
  const navigate = useNavigate()
  const [name, setName] = useState('')
  const [startDate, setStartDate] = useState('2023-01-01')
  const [endDate, setEndDate] = useState('2023-12-31')
  const [checkpointDays, setCheckpointDays] = useState(5)
  const [providers, setProviders] = useState<ProviderInfo[]>([])
  const [personalities, setPersonalities] = useState<PersonalityInfo[]>([])
  const [strategies, setStrategies] = useState<StrategyInfo[]>([])
  const [defaultPersonality, setDefaultPersonality] = useState('balanced')
  const [defaultPrompt, setDefaultPrompt] = useState('')
  const [agents, setAgents] = useState<AgentDraft[]>([])
  const [loadingProviders, setLoadingProviders] = useState(true)
  const [creating, setCreating] = useState(false)
  const [formError, setFormError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false
    setLoadingProviders(true)
    orchestratorApi
      .getProviders()
      .then((res) => {
        if (cancelled) return
        setProviders(res.providers)
        setPersonalities(res.personalities)
        setStrategies(res.strategies)
        setDefaultPersonality(res.default_personality)
        setDefaultPrompt(res.default_system_prompt)
        const firstAvailable = res.providers.find((p) => p.available) ?? res.providers[0]
        setAgents([
          newAgentDraft(res.default_personality, res.default_system_prompt, firstAvailable),
        ])
      })
      .catch((err) => {
        if (!cancelled) {
          setFormError(err instanceof Error ? err.message : 'Failed to load providers')
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingProviders(false)
      })
    return () => {
      cancelled = true
    }
  }, [])

  const availableProviders = useMemo(
    () => providers.filter((p) => p.available),
    [providers],
  )

  const personalityById = useMemo(
    () => Object.fromEntries(personalities.map((p) => [p.id, p])),
    [personalities],
  )

  const strategyById = useMemo(
    () => Object.fromEntries(strategies.map((s) => [s.id, s])),
    [strategies],
  )

  const updateAgent = useCallback((key: string, patch: Partial<AgentDraft>) => {
    setAgents((rows) => rows.map((row) => (row.key === key ? { ...row, ...patch } : row)))
  }, [])

  const applyPersonalityPrompt = useCallback(async (key: string, personalityId: string) => {
    try {
      const res = await orchestratorApi.getPersonalityPrompt(personalityId)
      updateAgent(key, {
        personality: personalityId,
        system_prompt: res.system_prompt,
        prompt_customized: false,
      })
    } catch (err) {
      setFormError(err instanceof Error ? err.message : 'Failed to load personality prompt')
    }
  }, [updateAgent])

  const onProviderChange = useCallback(
    (key: string, providerId: string) => {
      const spec = providers.find((p) => p.id === providerId)
      const model = spec?.models[0] ?? ''
      updateAgent(key, {
        provider: providerId,
        model,
      })
      if (providerId !== 'custom') {
        const agent = agents.find((a) => a.key === key)
        const personality = agent?.personality ?? defaultPersonality
        void applyPersonalityPrompt(key, personality)
      }
    },
    [agents, applyPersonalityPrompt, defaultPersonality, providers, updateAgent],
  )

  const onPersonalityChange = useCallback(
    (key: string, personalityId: string) => {
      void applyPersonalityPrompt(key, personalityId)
    },
    [applyPersonalityPrompt],
  )

  const addAgent = useCallback(() => {
    const first = availableProviders[0]
    setAgents((rows) => [
      ...rows,
      newAgentDraft(defaultPersonality, defaultPrompt, first),
    ])
  }, [availableProviders, defaultPersonality, defaultPrompt])

  const removeAgent = useCallback((key: string) => {
    setAgents((rows) => (rows.length <= 1 ? rows : rows.filter((r) => r.key !== key)))
  }, [])

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault()
      setCreating(true)
      setFormError(null)
      try {
        if (agents.some((a) => !a.name.trim())) {
          throw new Error('Each agent needs a name')
        }
        if (
          !Number.isInteger(checkpointDays) ||
          checkpointDays < 1 ||
          checkpointDays > 60
        ) {
          throw new Error('Days per segment must be a whole number between 1 and 60')
        }
        const result = await orchestratorApi.launchSimulation({
          name,
          start_date: startDate,
          end_date: endDate,
          checkpoint_interval_days: checkpointDays,
          agents: agents.map((a) => ({
            name: a.name.trim(),
            provider: a.provider,
            model: a.model,
            personality: a.provider === 'custom' ? undefined : a.personality,
            system_prompt:
              a.provider === 'custom' || !a.prompt_customized ? undefined : a.system_prompt,
          })),
        })
        onCreated()
        navigate(`/simulation/${result.simulation_id}`)
      } catch (err) {
        setFormError(err instanceof Error ? err.message : 'Launch failed')
      } finally {
        setCreating(false)
      }
    },
    [agents, checkpointDays, endDate, name, navigate, onCreated, startDate],
  )

  return (
    <Panel
      title="Create & launch simulation"
      subtitle="Pick agent personalities or rule-based strategies, then start the orchestrator"
    >
      {loadingProviders ? (
        <p className="empty-state">Loading available providers…</p>
      ) : (
        <form className="launch-form" onSubmit={handleSubmit}>
          <div className="launch-form-grid">
            <label>
              Name
              <input
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="Q1 2023 GPT nano run"
              />
            </label>
            <label>
              Start date
              <input
                required
                type="date"
                value={startDate}
                onChange={(e) => setStartDate(e.target.value)}
              />
            </label>
            <label>
              End date
              <input
                required
                type="date"
                value={endDate}
                onChange={(e) => setEndDate(e.target.value)}
              />
            </label>
            <label>
              Days per segment
              <input
                required
                type="number"
                min={1}
                max={60}
                value={checkpointDays}
                onChange={(e) => setCheckpointDays(Number(e.target.value))}
              />
              <span className="field-hint">Pause for approval after this many trading days (1–60)</span>
            </label>
          </div>

          <div className="agent-section-header">
            <h3>Agents</h3>
            <p className="lede">
              LLM agents use trading personalities (risk taker, momentum, etc.). Rules-based agents
              use Step 19 strategies — no API key required.
            </p>
            <button type="button" className="btn ghost" onClick={addAgent}>
              <Plus size={16} />
              Add agent
            </button>
          </div>

          <div className="agent-draft-list">
            {agents.map((agent, index) => {
              const spec = providers.find((p) => p.id === agent.provider)
              const isLlm = agent.provider !== 'custom'
              const personality = personalityById[agent.personality]
              const strategy = strategyById[agent.model]
              return (
                <div key={agent.key} className="agent-draft-card">
                  <div className="agent-draft-top">
                    <strong>Agent {index + 1}</strong>
                    {agents.length > 1 ? (
                      <button
                        type="button"
                        className="btn ghost icon-btn"
                        aria-label="Remove agent"
                        onClick={() => removeAgent(agent.key)}
                      >
                        <Trash2 size={14} />
                      </button>
                    ) : null}
                  </div>
                  <div className="launch-form-grid">
                    <label>
                      Display name
                      <input
                        required
                        value={agent.name}
                        onChange={(e) => updateAgent(agent.key, { name: e.target.value })}
                        placeholder="gpt-nano-trader"
                      />
                    </label>
                    <label>
                      Provider
                      <select
                        value={agent.provider}
                        onChange={(e) => onProviderChange(agent.key, e.target.value)}
                      >
                        {providers.map((p) => (
                          <option key={p.id} value={p.id} disabled={!p.available}>
                            {p.label}
                            {!p.available ? ' (no API key)' : ''}
                          </option>
                        ))}
                      </select>
                    </label>
                    {isLlm ? (
                      <label>
                        Trading personality
                        <select
                          value={agent.personality}
                          onChange={(e) => onPersonalityChange(agent.key, e.target.value)}
                        >
                          {personalities.map((p) => (
                            <option key={p.id} value={p.id}>
                              {p.label}
                            </option>
                          ))}
                        </select>
                        {personality ? (
                          <span className="field-hint">{personality.description}</span>
                        ) : null}
                      </label>
                    ) : (
                      <label>
                        Strategy
                        <select
                          value={agent.model}
                          onChange={(e) => updateAgent(agent.key, { model: e.target.value })}
                        >
                          {(spec?.models ?? []).map((m) => (
                            <option key={m} value={m}>
                              {strategyById[m]?.label ?? m}
                            </option>
                          ))}
                        </select>
                        {strategy ? (
                          <span className="field-hint">{strategy.description}</span>
                        ) : null}
                      </label>
                    )}
                    {isLlm ? (
                      <label>
                        Model
                        <select
                          value={agent.model}
                          onChange={(e) => updateAgent(agent.key, { model: e.target.value })}
                        >
                          {(spec?.models ?? []).map((m) => (
                            <option key={m} value={m}>
                              {m}
                            </option>
                          ))}
                        </select>
                      </label>
                    ) : null}
                  </div>
                  {isLlm ? (
                    <label className="prompt-field">
                      System prompt
                      <span className="field-hint">
                        Auto-filled from personality. Edit to override sizing and behavior rules.
                      </span>
                      <textarea
                        rows={6}
                        value={agent.system_prompt}
                        onChange={(e) =>
                          updateAgent(agent.key, {
                            system_prompt: e.target.value,
                            prompt_customized: true,
                          })
                        }
                      />
                    </label>
                  ) : (
                    <p className="agent-note">
                      Rules-based <strong>{strategy?.label ?? agent.model}</strong> agent — executes
                      deterministic strategy logic each tick.
                    </p>
                  )}
                </div>
              )
            })}
          </div>

          {formError ? <p className="form-error">{formError}</p> : null}
          <div className="form-actions">
            <button type="button" className="btn ghost" onClick={onCancel}>
              Cancel
            </button>
            <button
              type="submit"
              className="btn primary"
              disabled={creating || availableProviders.length === 0}
            >
              {creating ? 'Launching…' : 'Create & start simulation'}
            </button>
          </div>
        </form>
      )}
    </Panel>
  )
}
