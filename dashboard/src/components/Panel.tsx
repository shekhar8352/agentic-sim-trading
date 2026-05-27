import type { ReactNode } from 'react'
import './Panel.css'

interface PanelProps {
  title: string
  subtitle?: string
  action?: ReactNode
  children: ReactNode
  className?: string
}

export function Panel({ title, subtitle, action, children, className = '' }: PanelProps) {
  return (
    <section className={`panel ${className}`.trim()}>
      <header className="panel-header">
        <div>
          <h2 className="panel-title">{title}</h2>
          {subtitle ? <p className="panel-subtitle">{subtitle}</p> : null}
        </div>
        {action ? <div className="panel-action">{action}</div> : null}
      </header>
      <div className="panel-body">{children}</div>
    </section>
  )
}
