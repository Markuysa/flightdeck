import { LayoutGrid } from 'lucide-react'
import { EmptyState } from '../components/EmptyState'

/**
 * PLACEHOLDER — the Fleet screen (docs/DESIGN.md §4.1, all projects as
 * cards) ships in ticket 009-frontend-fleet. This route exists so `/` has
 * somewhere to render; nothing here should be built on by other screens.
 */
export function Fleet() {
  return (
    <div className="p-6">
      <EmptyState
        icon={LayoutGrid}
        title="Fleet"
        description="Project cards with status counts and autopilot state ship in ticket 009."
      />
    </div>
  )
}
