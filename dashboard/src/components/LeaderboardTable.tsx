import { Link } from 'react-router-dom'
import type { LeaderboardEntry } from '../api/types'
import { formatINR, formatPct } from '../lib/format'
import './LeaderboardTable.css'

interface LeaderboardTableProps {
  entries: LeaderboardEntry[]
  simId: string
}

export function LeaderboardTable({ entries, simId }: LeaderboardTableProps) {
  if (entries.length === 0) {
    return <p className="table-empty">No agents ranked yet.</p>
  }

  return (
    <div className="table-wrap">
      <table className="data-table">
        <thead>
          <tr>
            <th>Rank</th>
            <th>Agent</th>
            <th>Portfolio</th>
            <th>Return</th>
            <th>Sharpe</th>
            <th>Max DD</th>
          </tr>
        </thead>
        <tbody>
          {entries.map((e) => (
            <tr key={e.agent_id}>
              <td>
                <span className={`rank rank-${e.rank <= 3 ? e.rank : 'n'}`}>{e.rank}</span>
              </td>
              <td>
                <Link to={`/agent/${e.agent_id}?sim=${simId}`} className="agent-link">
                  {e.agent_name || e.agent_id.slice(0, 8)}
                </Link>
              </td>
              <td className="num">{formatINR(e.total_value)}</td>
              <td className={`num ${e.total_return_pct >= 0 ? 'profit' : 'loss'}`}>
                {formatPct(e.total_return_pct)}
              </td>
              <td className="num">{e.sharpe_ratio.toFixed(2)}</td>
              <td className="num loss">-{e.max_drawdown_pct.toFixed(1)}%</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
