import { ServerCrash, Trash2, UserPlus } from 'lucide-react'
import { useCallback, useEffect, useState } from 'react'
import { Link, useParams } from 'react-router-dom'
import { Button } from '../components/Button'
import { EmptyState } from '../components/EmptyState'
import { deleteAgent, listAgentConfigs, saveAgent } from '../lib/api'
import { ROLES, type Agent } from '../lib/types'

const INPUT_CLASS =
  'rounded-nested border border-border-soft bg-bg px-3 py-1.5 text-sm text-text placeholder:text-text-dim'

/**
 * The agent-configuration screen: one specialist per role, with the system
 * prompt and skills it works under.
 *
 * A ticket's `role` is what binds it to an agent, so the roster is presented as
 * the five roles rather than as a free list — every row answers "who takes
 * backend tickets here", which is the question the dispatcher asks. A role with
 * nobody configured is shown as such rather than hidden: an unstaffed role
 * still dispatches, just with no extra instructions, and knowing that is the
 * point of listing it.
 */
export function AgentConfig() {
  const { id } = useParams<{ id: string }>()
  const [agents, setAgents] = useState<Agent[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [editing, setEditing] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    if (!id) return
    try {
      setAgents(await listAgentConfigs(id))
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load agents.')
    } finally {
      setLoading(false)
    }
  }, [id])

  useEffect(() => {
    Promise.resolve()
      .then(() => refresh())
      .catch(() => {
        // refresh() already turns a failure into `error` state.
      })
  }, [refresh])

  if (!id) return null

  const byRole = new Map(agents.map((a) => [a.role, a]))

  return (
    <div className="flex flex-col gap-6 p-6">
      <header className="flex flex-col gap-1">
        <Link
          to={`/p/${encodeURIComponent(id)}`}
          className="w-fit text-xs text-text-mut transition-colors duration-150 hover:text-text"
        >
          ← Board
        </Link>
        <h1 className="font-display text-xl font-semibold text-text">Agents</h1>
        <p className="text-sm text-text-mut">
          Who takes which tickets, and how they work. A role with no agent still runs — it just
          gets no extra instructions.
        </p>
      </header>

      {error ? (
        <EmptyState
          icon={ServerCrash}
          title="Couldn't load agents"
          description={error}
          action={
            <Button variant="ghost" onClick={refresh}>
              Try again
            </Button>
          }
        />
      ) : loading ? (
        <p className="py-16 text-center text-sm text-text-mut" aria-live="polite">
          Loading agents…
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {ROLES.map((role) => (
            <RoleRow
              key={role}
              projectId={id}
              role={role}
              agent={byRole.get(role)}
              editing={editing === role}
              onEdit={() => setEditing(editing === role ? null : role)}
              onSaved={async () => {
                setEditing(null)
                await refresh()
              }}
              onRemoved={refresh}
            />
          ))}
        </ul>
      )}
    </div>
  )
}

function RoleRow({
  projectId,
  role,
  agent,
  editing,
  onEdit,
  onSaved,
  onRemoved,
}: {
  projectId: string
  role: string
  agent: Agent | undefined
  editing: boolean
  onEdit: () => void
  onSaved: () => void
  onRemoved: () => void
}) {
  const [name, setName] = useState(agent?.name ?? '')
  const [prompt, setPrompt] = useState(agent?.prompt ?? '')
  const [skills, setSkills] = useState((agent?.skills ?? []).join(', '))
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSave() {
    setSaving(true)
    setError(null)
    try {
      await saveAgent(projectId, {
        name: name.trim(),
        role,
        prompt: prompt.trim(),
        skills: skills
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      })
      onSaved()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save.')
    } finally {
      setSaving(false)
    }
  }

  async function handleRemove() {
    if (!agent) return
    try {
      await deleteAgent(projectId, agent.id)
      onRemoved()
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to remove.')
    }
  }

  return (
    <li className="flex flex-col gap-3 rounded-card border border-border-soft bg-surface p-4">
      <div className="flex flex-wrap items-center gap-3">
        <span className="w-20 shrink-0 rounded-chip border border-border-soft bg-surface-2 px-2 py-0.5 text-center text-[11px] font-medium uppercase tracking-wide text-text-mut">
          {role}
        </span>
        <span className="min-w-0 flex-1 truncate text-sm text-text">
          {agent ? (
            agent.name
          ) : (
            <span className="text-text-dim">No agent — dispatches with no extra instructions</span>
          )}
        </span>
        <div className="flex shrink-0 items-center gap-1">
          <Button variant="ghost" onClick={onEdit}>
            {editing ? 'Cancel' : agent ? 'Edit' : 'Configure'}
          </Button>
          {agent && (
            <button
              type="button"
              onClick={handleRemove}
              aria-label={`Remove the ${role} agent`}
              className="rounded-nested p-1.5 text-text-dim transition-colors duration-150 hover:bg-surface-2 hover:text-text"
            >
              <Trash2 className="h-4 w-4" />
            </button>
          )}
        </div>
      </div>

      {agent && !editing && agent.skills.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {agent.skills.map((skill) => (
            <span
              key={skill}
              className="rounded-chip border border-border-soft bg-surface-2 px-2 py-0.5 font-mono text-[11px] text-text-mut"
            >
              {skill}
            </span>
          ))}
        </div>
      )}

      {editing && (
        <div className="flex flex-col gap-3 border-t border-border-soft pt-3">
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            Name
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="Backend specialist"
              className={INPUT_CLASS}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            System prompt
            <textarea
              value={prompt}
              onChange={(e) => setPrompt(e.target.value)}
              rows={4}
              placeholder="Follow the repo's existing conventions. Write tests for every change. Never edit generated files."
              className={[INPUT_CLASS, 'leading-relaxed'].join(' ')}
            />
            <span className="text-xs text-text-dim">
              Sent with every ticket this agent takes.
            </span>
          </label>
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            Skills
            <input
              value={skills}
              onChange={(e) => setSkills(e.target.value)}
              placeholder="go, sqlite, testing"
              className={[INPUT_CLASS, 'font-mono'].join(' ')}
            />
            <span className="text-xs text-text-dim">
              Comma-separated. Passed to the routine as-is.
            </span>
          </label>

          {error && (
            <p role="alert" className="text-sm text-st-attention">
              {error}
            </p>
          )}

          <div className="flex gap-2">
            <Button onClick={handleSave} disabled={saving || !name.trim()}>
              <UserPlus className="h-4 w-4" />
              {saving ? 'Saving…' : 'Save'}
            </Button>
          </div>
        </div>
      )}
    </li>
  )
}
