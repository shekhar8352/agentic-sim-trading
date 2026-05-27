import { Link, Outlet, useLocation } from 'react-router-dom'
import { BarChart3, GitCompare, Layers, Radio } from 'lucide-react'
import './Layout.css'

const nav = [
  { to: '/simulations', label: 'Simulations', icon: Layers },
  { to: '/compare', label: 'Compare', icon: GitCompare },
]

export function Layout() {
  const location = useLocation()

  return (
    <div className="shell">
      <header className="topbar">
        <Link to="/simulations" className="brand">
          <span className="brand-mark" aria-hidden="true">
            <Radio size={18} strokeWidth={2.2} />
          </span>
          <span className="brand-text">
            <strong>Agentic</strong>
            <span>Sim Trading</span>
          </span>
        </Link>

        <nav className="nav" aria-label="Main">
          {nav.map(({ to, label, icon: Icon }) => {
            const active = location.pathname.startsWith(to)
            return (
              <Link
                key={to}
                to={to}
                className={`nav-link${active ? ' active' : ''}`}
              >
                <Icon size={16} aria-hidden="true" />
                {label}
              </Link>
            )
          })}
        </nav>

        <div className="topbar-meta">
          <BarChart3 size={14} aria-hidden="true" />
          <span>NSE Historical Sim</span>
        </div>
      </header>

      <main className="main">
        <Outlet />
      </main>
    </div>
  )
}
