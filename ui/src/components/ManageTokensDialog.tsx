import { useEffect, useRef, useState, type FormEvent } from 'react'
import { getSecretsStatus, setSecrets } from '../lib/api'
import type { SecretsStatus } from '../lib/types'
import { Button } from './Button'
import { INPUT_CLASS } from './RegisterProjectDialog'

export interface ManageTokensDialogProps {
  projectId: string
  projectName: string
  onClose: () => void
}

/** Set/rotate a project's tokens after registration (ticket 016): loads
 * GET .../secrets on mount to show a "set"/"not set" hint per token — never
 * the value — then PUTs only the fields the human filled in. Leaving a
 * field blank and submitting keeps that token unchanged (the backend's
 * overlay semantics). Follows RegisterProjectDialog's dialog conventions:
 * mounted conditionally by the caller, Escape/backdrop close it, errors
 * render inline without closing, focus starts on the first field and
 * returns to the trigger on close. Tokens are `type="password"`, never
 * logged, and held in state only until the request settles. */
export function ManageTokensDialog({ projectId, projectName, onClose }: ManageTokensDialogProps) {
  const [status, setStatus] = useState<SecretsStatus | null>(null)
  const [loadError, setLoadError] = useState<string | null>(null)
  const [routineToken, setRoutineToken] = useState('')
  const [githubToken, setGithubToken] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [saved, setSaved] = useState(false)
  const firstFieldRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null
    firstFieldRef.current?.focus()
    return () => previouslyFocused?.focus()
  }, [])

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  // Deferred via .then() (see useProjects.ts) so the mount-time fetch reads
  // as async to eslint-plugin-react-hooks' set-state-in-effect rule.
  useEffect(() => {
    Promise.resolve()
      .then(() => getSecretsStatus(projectId))
      .then(setStatus)
      .catch((err) => {
        setLoadError(err instanceof Error ? err.message : 'Failed to load token status.')
      })
  }, [projectId])

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSaved(false)
    setSubmitting(true)
    try {
      await setSecrets(projectId, {
        ...(routineToken.trim() ? { routine_token: routineToken.trim() } : {}),
        ...(githubToken.trim() ? { github_token: githubToken.trim() } : {}),
      })
      setRoutineToken('')
      setGithubToken('')
      setSaved(true)
      setStatus(await getSecretsStatus(projectId))
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to save tokens.')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-bg/80 p-4"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-labelledby="manage-tokens-title"
        className="w-full max-w-md rounded-card border border-border bg-surface p-6"
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id="manage-tokens-title" className="font-display text-lg font-semibold text-text">
          Manage tokens — {projectName}
        </h2>
        <form onSubmit={handleSubmit} className="mt-4 flex flex-col gap-4">
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            Routine token{' '}
            <span className="text-text-dim">({tokenHint(status?.routine_token_set)})</span>
            <input
              ref={firstFieldRef}
              type="password"
              autoComplete="off"
              value={routineToken}
              onChange={(event) => setRoutineToken(event.target.value)}
              placeholder="Leave blank to keep the current token"
              className={INPUT_CLASS}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            GitHub token{' '}
            <span className="text-text-dim">({tokenHint(status?.github_token_set)})</span>
            <input
              type="password"
              autoComplete="off"
              value={githubToken}
              onChange={(event) => setGithubToken(event.target.value)}
              placeholder="Leave blank to keep the current token"
              className={INPUT_CLASS}
            />
          </label>

          {loadError && (
            <p role="alert" className="text-sm text-st-attention">
              {loadError}
            </p>
          )}
          {error && (
            <p role="alert" className="text-sm text-st-attention">
              {error}
            </p>
          )}
          {saved && !error && <p className="text-sm text-st-done">Tokens saved.</p>}

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={onClose}>
              Close
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}

function tokenHint(isSet: boolean | undefined): string {
  if (isSet === undefined) return 'loading…'
  return isSet ? 'set' : 'not set'
}
