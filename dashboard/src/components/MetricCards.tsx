import type { AgentMetrics } from '../api/types'
import { formatPct } from '../lib/format'
import './MetricCards.css'

interface MetricCardsProps {
  metrics: AgentMetrics
}

const cards: {
  key: keyof AgentMetrics
  label: string
  format: (m: AgentMetrics) => string
  tone?: (m: AgentMetrics) => 'profit' | 'loss' | ''
}[] = [
  { key: 'total_return_pct', label: 'Total Return', format: (m) => formatPct(m.total_return_pct) },
  { key: 'sharpe_ratio', label: 'Sharpe Ratio', format: (m) => m.sharpe_ratio.toFixed(2) },
  { key: 'max_drawdown_pct', label: 'Max Drawdown', format: (m) => `-${m.max_drawdown_pct.toFixed(1)}%`, tone: () => 'loss' },
  { key: 'win_rate_pct', label: 'Win Rate (daily)', format: (m) => `${m.win_rate_pct.toFixed(1)}%` },
  { key: 'avg_holding_days', label: 'Avg Hold (est.)', format: (m) => `${m.avg_holding_days.toFixed(1)}d` },
  { key: 'total_filled_orders', label: 'Filled Orders', format: (m) => String(m.total_filled_orders) },
]

export function MetricCards({ metrics }: MetricCardsProps) {
  return (
    <div className="metric-grid">
      {cards.map(({ key, label, format, tone }) => (
        <div key={key} className="metric-card">
          <span className="metric-label">{label}</span>
          <span className={`metric-value${tone ? ` ${tone(metrics)}` : ''}`}>
            {format(metrics)}
          </span>
        </div>
      ))}
    </div>
  )
}
