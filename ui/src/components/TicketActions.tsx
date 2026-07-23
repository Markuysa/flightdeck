import { useState } from 'react'
import { approveTicket, dispatchTicket } from '../lib/api'
import type { DerivedStatus } from '../lib/types'
import { Button } from './Button'

export interface TicketActionsProps {
  projectId: string
  ticketId: number
  status: DerivedStatus
  className?: string
}

/**
 * The ticket action area (US-6, docs/DESIGN.md §4.3): Dispatch and
 * Approve-merge. Both only call the API and surface the result (a session
 * URL / the merge outcome) — neither ever dispatches or merges
 * client-side. Self-contained (owns its own request/result state) so it can
 * be mounted anywhere a `{projectId, ticketId, status}` is known — ticket
 * 012 (Agents) reuses this for its per-session Dispatch action.
 */
export function TicketActions({ projectId, ticketId, status, className = '' }: TicketActionsProps) {
  const [dispatching, setDispatching] = useState(false)
  const [sessionUrl, setSessionUrl] = useState<string | null>(null)
  const [dispatchError, setDispatchError] = useState<string | null>(null)

  const [approving, setApproving] = useState(false)
  const [approved, setApproved] = useState(false)
  const [approveError, setApproveError] = useState<string | null>(null)

  async function handleDispatch() {
    setDispatching(true)
    setDispatchError(null)
    setSessionUrl(null)
    try {
      const { session_url } = await dispatchTicket(projectId, ticketId)
      setSessionUrl(session_url)
    } catch (err) {
      setDispatchError(err instanceof Error ? err.message : 'Failed to dispatch ticket.')
    } finally {
      setDispatching(false)
    }
  }

  async function handleApprove() {
    setApproving(true)
    setApproveError(null)
    setApproved(false)
    try {
      await approveTicket(projectId, ticketId)
      setApproved(true)
    } catch (err) {
      setApproveError(err instanceof Error ? err.message : 'Failed to approve merge.')
    } finally {
      setApproving(false)
    }
  }

  return (
    <div className={['flex flex-col gap-3', className].filter(Boolean).join(' ')}>
      <div className="flex flex-wrap items-center gap-3">
        <Button onClick={handleDispatch} disabled={status !== 'ready' || dispatching}>
          {dispatching ? 'Dispatching…' : 'Dispatch'}
        </Button>
        {status === 'in_review' && (
          <Button variant="ghost" onClick={handleApprove} disabled={approving}>
            {approving ? 'Merging…' : 'Approve merge'}
          </Button>
        )}
      </div>

      {sessionUrl && (
        <p className="text-sm text-st-done">
          Session started:{' '}
          <a
            href={sessionUrl}
            target="_blank"
            rel="noreferrer"
            className="font-mono underline-offset-2 hover:underline"
          >
            {sessionUrl}
          </a>
        </p>
      )}
      {dispatchError && (
        <p role="alert" className="text-sm text-st-attention">
          {dispatchError}
        </p>
      )}
      {approved && <p className="text-sm text-st-done">Merged.</p>}
      {approveError && (
        <p role="alert" className="text-sm text-st-attention">
          {approveError}
        </p>
      )}
    </div>
  )
}
