export function formatINR(value: number): string {
  return new Intl.NumberFormat('en-IN', {
    style: 'currency',
    currency: 'INR',
    maximumFractionDigits: 0,
  }).format(value)
}

export function formatPct(value: number, digits = 2): string {
  const sign = value > 0 ? '+' : ''
  return `${sign}${value.toFixed(digits)}%`
}

export function formatNumber(value: number, digits = 2): string {
  return value.toLocaleString('en-IN', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  })
}

export function shortId(id: string): string {
  return id.slice(0, 8)
}

export function statusLabel(status: string): string {
  return status.replace(/_/g, ' ')
}

export const CHART_COLORS = [
  '#e8952f',
  '#4ade80',
  '#38bdf8',
  '#f472b6',
  '#a78bfa',
  '#fbbf24',
  '#2dd4bf',
  '#fb7185',
]
