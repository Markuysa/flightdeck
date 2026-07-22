import { FileText } from 'lucide-react'
import { useParams } from 'react-router-dom'
import { EmptyState } from '../components/EmptyState'

/**
 * PLACEHOLDER — the Ticket screen (docs/DESIGN.md §4.3, body/acceptance
 * criteria/dependency trail/PR/CI) ships in ticket 011-frontend-ticket. This
 * route exists so `/p/:id/t/:tid` has somewhere to render; nothing here
 * should be built on by other screens.
 */
export function Ticket() {
  const { id, tid } = useParams<{ id: string; tid: string }>()
  return (
    <div className="p-6">
      <EmptyState
        icon={FileText}
        title={`Ticket ${tid} — ${id}`}
        description="Body, acceptance criteria and the dependency trail ship in ticket 011."
      />
    </div>
  )
}
