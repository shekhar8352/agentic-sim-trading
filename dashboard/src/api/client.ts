import type {
  AgentEquityCurve,
  AgentMetrics,
  AgentOrder,
  AgentSummary,
  LeaderboardEntry,
  OrderRow,
  PortfolioDetail,
  SimulationDetail,
  SimulationSummary,
} from './types'

const baseUrl = import.meta.env.VITE_API_URL ?? ''

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${baseUrl}${path}`, {
    headers: { Accept: 'application/json', ...init?.headers },
    ...init,
  })
  if (!res.ok) {
    const body = await res.text()
    throw new Error(body || `HTTP ${res.status}`)
  }
  return res.json() as Promise<T>
}

export const api = {
  listSimulations: () =>
    request<{ simulations: SimulationSummary[] }>('/api/v1/simulations'),

  createSimulation: (body: {
    name: string
    start_date: string
    end_date: string
    config?: Record<string, unknown>
  }) =>
    request<{ id: string; status: string }>('/api/v1/simulations', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    }),

  getSimulation: (id: string) =>
    request<SimulationDetail>(`/api/v1/simulations/${id}`),

  startSimulation: (id: string) =>
    request<{ detail: string }>(`/api/v1/simulations/${id}/start`, { method: 'POST' }),

  pauseSimulation: (id: string) =>
    request<{ detail: string }>(`/api/v1/simulations/${id}/pause`, { method: 'POST' }),

  proceedSimulation: (id: string) =>
    request<{ detail: string }>(`/api/v1/simulations/${id}/proceed`, { method: 'POST' }),

  getLeaderboard: (simId: string) =>
    request<{ simulation_id: string; entries: LeaderboardEntry[] }>(
      `/api/v1/leaderboard/${simId}`,
    ),

  getEquityCurves: (simId: string) =>
    request<{ simulation_id: string; curves: AgentEquityCurve[] }>(
      `/api/v1/dashboard/simulations/${simId}/equity-curves`,
    ),

  getRecentOrders: (simId: string, limit = 20) =>
    request<{ simulation_id: string; orders: OrderRow[] }>(
      `/api/v1/dashboard/simulations/${simId}/orders?limit=${limit}`,
    ),

  listAgents: (simId: string) =>
    request<{ simulation_id: string; agents: AgentSummary[] }>(
      `/api/v1/dashboard/simulations/${simId}/agents`,
    ),

  getAgentPortfolio: (simId: string, agentId: string) =>
    request<PortfolioDetail>(
      `/api/v1/dashboard/simulations/${simId}/agents/${agentId}/portfolio`,
    ),

  getAgentOrders: (simId: string, agentId: string, limit = 100) =>
    request<AgentOrder[]>(
      `/api/v1/dashboard/simulations/${simId}/agents/${agentId}/orders?limit=${limit}`,
    ),

  getAgentMetrics: (simId: string, agentId: string) =>
    request<AgentMetrics>(
      `/api/v1/dashboard/simulations/${simId}/agents/${agentId}/metrics`,
    ),
}
