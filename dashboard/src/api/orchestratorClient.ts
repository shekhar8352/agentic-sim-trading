import type { ProviderInfo } from './types'

const orchestratorBase = import.meta.env.VITE_ORCHESTRATOR_URL ?? '/orchestrator-api'

async function orchRequest<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${orchestratorBase}${path}`, {
    headers: { Accept: 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(body || `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

export interface LaunchAgentInput {
  name: string
  provider: string
  model: string
  system_prompt?: string
}

export const orchestratorApi = {
  getProviders: () =>
    orchRequest<{ providers: ProviderInfo[]; default_system_prompt: string }>(
      '/api/v1/providers',
    ),

  launchSimulation: (body: {
    name: string
    start_date: string
    end_date: string
    checkpoint_interval_days: number
    agents: LaunchAgentInput[]
  }) =>
    orchRequest<{ simulation_id: string; status: string; agents: LaunchAgentInput[] }>(
      '/api/v1/simulations/launch',
      {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      },
    ),

  updatePrompts: (simulationId: string, agents: { agent_id: string; system_prompt: string }[]) =>
    orchRequest<{ simulation_id: string; updated: number; live_agents_updated: number }>(
      `/api/v1/simulations/${simulationId}/prompts`,
      {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ agents }),
      },
    ),

  orchestratorStatus: (simulationId: string) =>
    orchRequest<{ simulation_id: string; running: boolean; agent_count: number }>(
      `/api/v1/orchestrator/status/${simulationId}`,
    ),
}
