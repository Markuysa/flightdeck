import { Users } from 'lucide-react'
import { EmptyState } from '../components/EmptyState'

/**
 * PLACEHOLDER — the Agents screen (docs/DESIGN.md §4.4, live sessions) ships
 * in ticket 012-frontend-agents-actions. This route exists so `/agents` has
 * somewhere to render; nothing here should be built on by other screens.
 */
export function Agents() {
  return (
    <div className="p-6">
      <EmptyState
        icon={Users}
        title="Agents"
        description="Live agent sessions — who is working, on what — ship in ticket 012."
      />
    </div>
  )
}
