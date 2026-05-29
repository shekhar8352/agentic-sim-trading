import { Link } from 'react-router-dom'
import type { OrderRow } from '../api/types'
import { StatusBadge } from './StatusBadge'
import './OrderFeed.css'

interface OrderFeedProps {
  orders: OrderRow[]
  simId: string
}

export function OrderFeed({ orders, simId }: OrderFeedProps) {
  if (orders.length === 0) {
    return <p className="feed-empty">No orders yet.</p>
  }

  return (
    <ul className="order-feed">
      {orders.map((o) => (
        <li key={o.id} className="order-item">
          <div className="order-main">
            <span className={`side side-${o.side}`}>{o.side.toUpperCase()}</span>
            <span className="symbol">{o.symbol}</span>
            <span className="qty">×{o.quantity}</span>
          </div>
          <div className="order-meta">
            <Link to={`/agent/${o.agent_id}?sim=${simId}`} className="order-agent">
              {o.agent_name || o.agent_id.slice(0, 8)}
            </Link>
            <StatusBadge status={o.status} />
            <time dateTime={o.created_at}>
              {new Date(o.created_at).toLocaleTimeString('en-IN', {
                hour: '2-digit',
                minute: '2-digit',
              })}
            </time>
          </div>
        </li>
      ))}
    </ul>
  )
}
