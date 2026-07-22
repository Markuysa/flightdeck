import { useState, type ComponentType } from 'react'
import { NavLink, Outlet } from 'react-router-dom'
import { LayoutGrid, PanelLeftClose, PanelLeftOpen, Users } from 'lucide-react'

interface NavItem {
  to: string
  label: string
  icon: ComponentType<{ className?: string }>
  end?: boolean
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Fleet', icon: LayoutGrid, end: true },
  { to: '/agents', label: 'Agents', icon: Users },
]

/**
 * App shell: a sidebar with Fleet/Agents nav that collapses to a top bar at
 * the `sidebar` breakpoint (860px, docs/DESIGN.md §2.4), and manually
 * collapses to icon-only above it. Dark-only, per §1.
 */
export function Shell() {
  const [collapsed, setCollapsed] = useState(false)

  return (
    <div className="flex min-h-screen flex-col bg-bg text-text sidebar:h-screen sidebar:flex-row">
      <aside
        className={[
          'flex shrink-0 flex-col border-b border-border-soft bg-surface',
          'sidebar:h-screen sidebar:border-b-0 sidebar:border-r sidebar:transition-[width] sidebar:duration-150',
          collapsed ? 'sidebar:w-16' : 'sidebar:w-60',
        ].join(' ')}
      >
        <div className="flex items-center justify-between gap-2 px-4 py-3 sidebar:flex-col sidebar:items-stretch">
          <span className="truncate font-display text-sm font-semibold tracking-wide">
            {collapsed ? 'FD' : 'FlightDeck'}
          </span>
          <button
            type="button"
            onClick={() => setCollapsed((c) => !c)}
            aria-label={collapsed ? 'Expand sidebar' : 'Collapse sidebar'}
            aria-pressed={collapsed}
            className="hidden rounded-nested p-1.5 text-text-mut transition-colors duration-150 hover:bg-surface-2 hover:text-text sidebar:inline-flex"
          >
            {collapsed ? (
              <PanelLeftOpen className="h-4 w-4" />
            ) : (
              <PanelLeftClose className="h-4 w-4" />
            )}
          </button>
        </div>

        <nav className="flex flex-1 gap-1 px-2 pb-3 sidebar:flex-col sidebar:pb-4">
          {NAV_ITEMS.map(({ to, label, icon: Icon, end }) => (
            <NavLink
              key={to}
              to={to}
              end={end}
              className={({ isActive }) =>
                [
                  'flex items-center gap-2.5 rounded-nested px-3 py-2 text-sm font-medium transition-colors duration-150',
                  isActive
                    ? 'bg-accent-soft text-accent'
                    : 'text-text-mut hover:bg-surface-2 hover:text-text',
                ].join(' ')
              }
            >
              <Icon className="h-4 w-4 shrink-0" />
              <span className={collapsed ? 'sidebar:hidden' : ''}>{label}</span>
            </NavLink>
          ))}
        </nav>
      </aside>

      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  )
}
