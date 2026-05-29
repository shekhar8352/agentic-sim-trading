import './StatusBadge.css'

const toneMap: Record<string, string> = {
  running: 'live',
  paused: 'idle',
  checkpoint: 'live',
  completed: 'done',
  filled: 'profit',
  rejected: 'loss',
  pending: 'idle',
  canceled: 'muted',
}

interface StatusBadgeProps {
  status: string
}

export function StatusBadge({ status }: StatusBadgeProps) {
  const tone = toneMap[status.toLowerCase()] ?? 'muted'
  return (
    <span className={`badge badge-${tone}`}>
      {status}
    </span>
  )
}
