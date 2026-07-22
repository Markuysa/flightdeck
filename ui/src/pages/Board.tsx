import { Kanban } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { EmptyState } from '../components/EmptyState'

/**
 * PLACEHOLDER — the Board screen (docs/DESIGN.md §4.2, per-project kanban)
 * ships in ticket 010-frontend-board. This route exists so `/p/:id` has
 * somewhere to render; nothing here should be built on by other screens.
 */
export function Board() {
  const { id } = useParams<{ id: string }>()
  return (
    <div className="p-6">
      <EmptyState
        icon={Kanban}
        title={`Board — ${id}`}
        description="Kanban columns by derived status ship in ticket 010."
      />
    </div>
  )
}
