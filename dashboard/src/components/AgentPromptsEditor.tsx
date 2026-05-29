import { useCallback, useEffect, useState } from 'react'
import { orchestratorApi } from '../api/orchestratorClient'
import type { OrchestratorAgentConfig } from '../api/types'
import { Panel } from './Panel'

interface AgentPromptsEditorProps {
  simulationId: string
  agents: OrchestratorAgentConfig[]
  onSaved?: () => void
}

export function AgentPromptsEditor({
  simulationId,
  agents,
  onSaved,
}: AgentPromptsEditorProps) {
  const [drafts, setDrafts] = useState<OrchestratorAgentConfig[]>(agents)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState<string | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    setDrafts(agents)
  }, [agents])

  const updatePrompt = useCallback((agentId: string, system_prompt: string) => {
    setDrafts((rows) =>
      rows.map((row) => (row.agent_id === agentId ? { ...row, system_prompt } : row)),
    )
  }, [])

  const handleSave = useCallback(async () => {
    setSaving(true)
    setError(null)
    setMessage(null)
    try {
      const llmAgents = drafts.filter((a) => a.provider !== 'custom')
      const result = await orchestratorApi.updatePrompts(
        simulationId,
        llmAgents.map((a) => ({
          agent_id: a.agent_id,
          system_prompt: a.system_prompt ?? '',
        })),
      )
      setMessage(
        `Saved ${result.updated} prompt(s)` +
          (result.live_agents_updated
            ? ` · ${result.live_agents_updated} live agent(s) updated`
            : ''),
      )
      onSaved?.()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Save failed')
    } finally {
      setSaving(false)
    }
  }, [drafts, onSaved, simulationId])

  if (drafts.length === 0) return null

  return (
    <Panel title="Agent prompts" subtitle="Edit system prompts for LLM agents in this run">
      <div className="agent-draft-list">
        {drafts.map((agent) => (
          <div key={agent.agent_id} className="agent-draft-card">
            <div className="agent-draft-top">
              <strong>{agent.name}</strong>
              <span className="tick-label">
                {agent.provider} · {agent.model}
              </span>
            </div>
            {agent.provider === 'custom' ? (
              <p className="agent-note">Rules-based agent — no prompt to edit.</p>
            ) : (
              <label className="prompt-field">
                System prompt
                <textarea
                  rows={5}
                  value={agent.system_prompt ?? ''}
                  onChange={(e) => updatePrompt(agent.agent_id, e.target.value)}
                />
              </label>
            )}
          </div>
        ))}
      </div>
      {error ? <p className="form-error">{error}</p> : null}
      {message ? <p className="save-note">{message}</p> : null}
      <div className="form-actions">
        <button
          type="button"
          className="btn primary"
          disabled={saving || drafts.every((a) => a.provider === 'custom')}
          onClick={handleSave}
        >
          {saving ? 'Saving…' : 'Save prompts'}
        </button>
      </div>
    </Panel>
  )
}
