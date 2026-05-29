import { Cell, Pie, PieChart, ResponsiveContainer, Tooltip } from 'recharts'
import type { HoldingDetail } from '../api/types'
import { CHART_COLORS, formatINR } from '../lib/format'
import './HoldingsPieChart.css'

interface HoldingsPieChartProps {
  holdings: HoldingDetail[]
  cash: number
}

export function HoldingsPieChart({ holdings, cash }: HoldingsPieChartProps) {
  const slices = [
    ...holdings.map((h) => ({
      name: h.symbol,
      value: h.position_value,
    })),
    ...(cash > 0 ? [{ name: 'Cash', value: cash }] : []),
  ]

  if (slices.length === 0) {
    return <p className="pie-empty">No holdings.</p>
  }

  return (
    <div className="holdings-pie">
      <ResponsiveContainer width="100%" height={260}>
        <PieChart>
          <Pie
            data={slices}
            dataKey="value"
            nameKey="name"
            cx="50%"
            cy="50%"
            innerRadius={56}
            outerRadius={96}
            paddingAngle={2}
            stroke="var(--bg-surface)"
            strokeWidth={2}
          >
            {slices.map((_, i) => (
              <Cell key={i} fill={CHART_COLORS[i % CHART_COLORS.length]} />
            ))}
          </Pie>
          <Tooltip
            formatter={(value) => formatINR(Number(value))}
            contentStyle={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-strong)',
              borderRadius: 'var(--radius-sm)',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
            }}
          />
        </PieChart>
      </ResponsiveContainer>
      <ul className="pie-legend">
        {slices.map((s, i) => (
          <li key={s.name}>
            <span className="dot" style={{ background: CHART_COLORS[i % CHART_COLORS.length] }} />
            {s.name}
          </li>
        ))}
      </ul>
    </div>
  )
}
