export interface SimulationSummary {
  id: string
  name: string
  start_date: string
  end_date: string
  as_of_date?: string
  status: string
  config?: Record<string, unknown>
  created_at: string
}

export interface OrchestratorAgentConfig {
  agent_id: string
  name: string
  provider: string
  model: string
  system_prompt?: string
}

export interface ProviderInfo {
  id: string
  label: string
  available: boolean
  models: string[]
  requires_api_key: boolean
}

export interface PersonalityInfo {
  id: string
  label: string
  description: string
  applies_to: string
}

export interface StrategyInfo {
  id: string
  label: string
  description: string
}

export interface SimulationDetail {
  id: string
  name: string
  start_date: string
  end_date: string
  db_status: string
  as_of_date?: string
  current_trading_day?: string
  clock_status?: string
  clock_loaded?: boolean
  trading_day_index?: number
  total_trading_days?: number
  tick_speed_multiplier?: number
  checkpoint_interval_days?: number
  days_since_checkpoint?: number
  awaiting_proceed?: boolean
  auto_tick_enabled?: boolean
  tick_interval_seconds?: number
  config?: {
    orchestrator_agents?: OrchestratorAgentConfig[]
    checkpoint_interval_days?: number
    [key: string]: unknown
  }
}

export interface LeaderboardEntry {
  rank: number
  agent_id: string
  agent_name: string
  snapshot_date?: string
  total_value: number
  cash: number
  invested_value: number
  total_return_pct: number
  sharpe_ratio: number
  max_drawdown_pct: number
}

export interface EquityPoint {
  date: string
  total_value: number
  cash: number
  invested_value: number
  return_pct: number
}

export interface AgentEquityCurve {
  agent_id: string
  agent_name: string
  points: EquityPoint[]
}

export interface OrderRow {
  id: string
  agent_id: string
  agent_name: string
  symbol: string
  side: string
  quantity: number
  status: string
  filled_price?: number
  match_on_date?: string
  rejection_reason?: string
  created_at: string
}

export interface AgentSummary {
  agent_id: string
  agent_name: string
  model?: string
}

export interface HoldingDetail {
  symbol: string
  quantity: number
  avg_buy_price: number
  mark_price: number
  position_value: number
  unrealized_pnl: number
}

export interface PortfolioDetail {
  simulation_id: string
  agent_id: string
  as_of_date: string
  cash: number
  holdings: HoldingDetail[]
  invested_value: number
  total_value: number
  total_pnl: number
  total_return_pct: number
  starting_capital: number
}

export interface AgentMetrics {
  agent_id: string
  agent_name: string
  total_return_pct: number
  sharpe_ratio: number
  max_drawdown_pct: number
  win_rate_pct: number
  avg_holding_days: number
  total_filled_orders: number
  total_rejected_orders: number
}

export interface AgentOrder {
  id: string
  symbol: string
  order_type: string
  side: string
  quantity: number
  status: string
  filled_price?: number
  filled_at?: string
  match_on_date?: string
  rejection_reason?: string
  fees_total?: number
  trade_value?: number
  created_at: string
}
