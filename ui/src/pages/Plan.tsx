import { Check, ServerCrash, Sparkles } from 'lucide-react'
import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { Button } from '../components/Button'
import { EmptyState } from '../components/EmptyState'
import { applyPlan, proposePlan } from '../lib/api'
import { statusColor } from '../lib/status'
import type { Applied, Plan as PlanData, PlanTicket } from '../lib/types'

/**
 * The Plan screen: describe a goal, read what the model proposes, then apply
 * it.
 *
 * The review step is the whole point of the screen. Applying writes tickets an
 * autonomous scheduler will start dispatching, so the plan is shown in full —
 * every ticket's body, criteria and dependencies — before anything is written.
 * A one-click "generate and go" would be faster and would remove the only
 * moment a person can catch a bad decomposition.
 */
export function Plan() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()

  const [goal, setGoal] = useState('')
  const [plan, setPlan] = useState<PlanData | null>(null)
  const [applied, setApplied] = useState<Applied | null>(null)
  const [planning, setPlanning] = useState(false)
  const [applying, setApplying] = useState(false)
  const [error, setError] = useState<string | null>(null)

  if (!id) return null

  async function handlePropose(event: React.FormEvent) {
    event.preventDefault()
    if (!id || !goal.trim()) return
    setPlanning(true)
    setError(null)
    setPlan(null)
    setApplied(null)
    try {
      setPlan(await proposePlan(id, goal.trim()))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to plan.')
    } finally {
      setPlanning(false)
    }
  }

  async function handleApply() {
    if (!id || !plan) return
    setApplying(true)
    setError(null)
    try {
      setApplied(await applyPlan(id, plan))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to apply the plan.')
    } finally {
      setApplying(false)
    }
  }

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex flex-col gap-1">
        <Link
          to={`/p/${encodeURIComponent(id)}`}
          className="w-fit text-xs text-text-mut transition-colors duration-150 hover:text-text"
        >
          ← Board
        </Link>
        <h1 className="font-display text-xl font-semibold text-text">Plan work</h1>
        <p className="text-sm text-text-mut">
          Describe what you want built. You&rsquo;ll review the tickets before anything is written.
        </p>
      </header>

      <form onSubmit={handlePropose} className="flex flex-col gap-3">
        <label className="flex flex-col gap-1.5 text-sm text-text-mut" htmlFor="plan-goal">
          Goal
        </label>
        <textarea
          id="plan-goal"
          value={goal}
          onChange={(event) => setGoal(event.target.value)}
          rows={4}
          placeholder="Add invoicing: customers can create, send and track invoices, with PDF export."
          className="rounded-nested border border-border-soft bg-bg px-3 py-2 text-sm leading-relaxed text-text placeholder:text-text-dim"
        />
        <div className="flex items-center gap-3">
          <Button type="submit" disabled={planning || !goal.trim()}>
            <Sparkles className="h-4 w-4" />
            {planning ? 'Planning…' : 'Plan it'}
          </Button>
          {plan && (
            <span className="font-mono text-xs text-text-dim">
              {plan.tickets.length} tickets · {plan.model}
            </span>
          )}
        </div>
      </form>

      {error && (
        <EmptyState icon={ServerCrash} title="Couldn't plan this" description={error} />
      )}

      {applied && (
        <section className="flex flex-col gap-3 rounded-card border border-st-done/40 bg-surface p-4">
          <h2 className="flex items-center gap-2 font-display text-base font-semibold text-st-done">
            <Check className="h-4 w-4" />
            Wrote {applied.ids.length} tickets
          </h2>
          <ul className="flex flex-col gap-1 font-mono text-xs text-text-mut">
            {applied.files.map((file) => (
              <li key={file}>{file}</li>
            ))}
          </ul>
          <div>
            <Button variant="ghost" onClick={() => navigate(`/p/${encodeURIComponent(id!)}`)}>
              Open the board
            </Button>
          </div>
        </section>
      )}

      {plan && !applied && (
        <section className="flex flex-col gap-4">
          <div className="rounded-card border border-border-soft bg-surface p-4">
            <h2 className="font-mono text-[10.5px] uppercase tracking-[0.1em] text-text-dim">
              Approach
            </h2>
            <p className="mt-2 text-sm leading-relaxed text-text">{plan.summary}</p>
          </div>

          <ol className="flex flex-col gap-2">
            {plan.tickets.map((ticket) => (
              <PlanTicketCard key={ticket.number} ticket={ticket} />
            ))}
          </ol>

          <div className="flex flex-wrap items-center gap-3 border-t border-border-soft pt-4">
            <Button onClick={handleApply} disabled={applying}>
              {applying ? 'Writing…' : `Write ${plan.tickets.length} tickets`}
            </Button>
            <Button variant="ghost" onClick={() => setPlan(null)} disabled={applying}>
              Discard
            </Button>
            <p className="text-xs text-text-dim">
              Writes to the project&rsquo;s <span className="font-mono">docs/tickets/</span>. Nothing
              is committed or dispatched.
            </p>
          </div>
        </section>
      )}
    </div>
  )
}

function PlanTicketCard({ ticket }: { ticket: PlanTicket }) {
  return (
    <li
      // The left edge carries `ready`'s colour for a ticket nothing blocks and
      // `blocked`'s for one that waits — the same reading the board will give
      // these tickets once they exist, shown before they are written.
      style={{ borderLeftColor: statusColor(ticket.depends.length === 0 ? 'ready' : 'blocked') }}
      className="flex flex-col gap-2 rounded-card border border-l-2 border-border-soft bg-surface p-4"
    >
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-mono text-xs text-text-dim">#{ticket.number}</span>
        <h3 className="flex-1 text-sm font-medium text-text">{ticket.title}</h3>
        <span className="rounded-chip border border-border-soft bg-surface-2 px-2 py-0.5 text-[11px] font-medium uppercase tracking-wide text-text-mut">
          {ticket.role}
        </span>
      </div>

      {ticket.body && (
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-text-mut">{ticket.body}</p>
      )}

      {ticket.acceptance.length > 0 && (
        <ul className="flex flex-col gap-1">
          {ticket.acceptance.map((criterion) => (
            <li key={criterion} className="flex gap-2 text-xs text-text-mut">
              <span className="text-text-dim">□</span>
              {criterion}
            </li>
          ))}
        </ul>
      )}

      <div className="flex flex-wrap items-center gap-3 text-xs text-text-dim">
        {ticket.depends.length > 0 ? (
          <span>
            Waits for{' '}
            <span className="font-mono">{ticket.depends.map((d) => `#${d}`).join(', ')}</span>
          </span>
        ) : (
          <span>Starts immediately</span>
        )}
      </div>
    </li>
  )
}
