import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import type { AgentEquityCurve } from '../api/types'
import { CHART_COLORS, formatINR } from '../lib/format'
import './EquityChart.css'

interface EquityChartProps {
  curves: AgentEquityCurve[]
}

export function EquityChart({ curves }: EquityChartProps) {
  if (curves.length === 0) {
    return <p className="empty-chart">No equity data yet — start the simulation to populate curves.</p>
  }

  const dateSet = new Set<string>()
  curves.forEach((c) => c.points.forEach((p) => dateSet.add(p.date)))
  const dates = [...dateSet].sort()

  const data = dates.map((date) => {
    const row: Record<string, string | number> = { date }
    curves.forEach((curve) => {
      const pt = curve.points.find((p) => p.date === date)
      if (pt) row[curve.agent_name || curve.agent_id.slice(0, 8)] = pt.total_value
    })
    return row
  })

  return (
    <div className="equity-chart">
      <ResponsiveContainer width="100%" height={320}>
        <LineChart data={data} margin={{ top: 8, right: 12, left: 8, bottom: 0 }}>
          <CartesianGrid stroke="var(--border)" strokeDasharray="3 3" vertical={false} />
          <XAxis
            dataKey="date"
            tick={{ fill: 'var(--text-dim)', fontSize: 10, fontFamily: 'var(--font-mono)' }}
            tickLine={false}
            axisLine={{ stroke: 'var(--border)' }}
            minTickGap={40}
          />
          <YAxis
            tick={{ fill: 'var(--text-dim)', fontSize: 10, fontFamily: 'var(--font-mono)' }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v) => `₹${(v / 100000).toFixed(1)}L`}
            width={56}
          />
          <Tooltip
            contentStyle={{
              background: 'var(--bg-elevated)',
              border: '1px solid var(--border-strong)',
              borderRadius: 'var(--radius-sm)',
              fontFamily: 'var(--font-mono)',
              fontSize: 12,
            }}
            formatter={(value) => formatINR(Number(value))}
            labelStyle={{ color: 'var(--text-muted)' }}
          />
          <Legend
            wrapperStyle={{ fontFamily: 'var(--font-mono)', fontSize: 11, paddingTop: 12 }}
          />
          {curves.map((curve, i) => (
            <Line
              key={curve.agent_id}
              type="monotone"
              dataKey={curve.agent_name || curve.agent_id.slice(0, 8)}
              stroke={CHART_COLORS[i % CHART_COLORS.length]}
              strokeWidth={2}
              dot={false}
              activeDot={{ r: 4 }}
            />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </div>
  )
}
