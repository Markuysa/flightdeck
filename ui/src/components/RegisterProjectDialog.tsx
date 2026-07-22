import { useEffect, useRef, useState, type FormEvent } from 'react'
import type { CreateProjectRequest } from '../lib/types'
import { Button } from './Button'

export interface RegisterProjectDialogProps {
  onClose: () => void
  /** Rejects with the API's validation/conflict message on failure — shown
   * inline, the dialog stays open so the human can fix the fields. */
  onRegister: (body: CreateProjectRequest) => Promise<void>
}

const INPUT_CLASS =
  'rounded-nested border border-border-soft bg-bg px-3 py-1.5 text-sm text-text placeholder:text-text-dim'

/** Register a project (US-7): posts `{name, repo_path, github?}`. Backend
 * validation errors (400/409/...) render inline rather than closing the
 * dialog (docs/tickets/009's acceptance criteria). Escape and a backdrop
 * click both close it; focus starts on the name field and returns to
 * whatever triggered the dialog on close.
 *
 * The caller mounts this only while the dialog should be open (`{open &&
 * <RegisterProjectDialog .../>}`) rather than passing an `open` prop — a
 * fresh mount is a fresh, blank form for free, no reset effect needed. */
export function RegisterProjectDialog({ onClose, onRegister }: RegisterProjectDialogProps) {
  const [name, setName] = useState('')
  const [repoPath, setRepoPath] = useState('')
  const [owner, setOwner] = useState('')
  const [repo, setRepo] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const nameRef = useRef<HTMLInputElement>(null)

  // Focus the first field on mount; return focus to whatever triggered the
  // dialog once it unmounts. No state to set here, so nothing for
  // react-hooks/set-state-in-effect to flag.
  useEffect(() => {
    const previouslyFocused = document.activeElement as HTMLElement | null
    nameRef.current?.focus()
    return () => previouslyFocused?.focus()
  }, [])

  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [onClose])

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (Boolean(owner.trim()) !== Boolean(repo.trim())) {
      setError('Provide both a GitHub owner and repo, or leave both blank.')
      return
    }

    setError(null)
    setSubmitting(true)
    try {
      await onRegister({
        name: name.trim(),
        repo_path: repoPath.trim(),
        ...(owner.trim() && repo.trim() ? { github: { owner: owner.trim(), repo: repo.trim() } } : {}),
      })
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to register project.')
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
        aria-labelledby="register-project-title"
        className="w-full max-w-md rounded-card border border-border bg-surface p-6"
        onClick={(event) => event.stopPropagation()}
      >
        <h2 id="register-project-title" className="font-display text-lg font-semibold text-text">
          Register project
        </h2>
        <form onSubmit={handleSubmit} className="mt-4 flex flex-col gap-4">
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            Name
            <input
              ref={nameRef}
              required
              value={name}
              onChange={(event) => setName(event.target.value)}
              className={INPUT_CLASS}
            />
          </label>
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            Repository path
            <input
              required
              value={repoPath}
              onChange={(event) => setRepoPath(event.target.value)}
              placeholder="/path/to/repo"
              className={[INPUT_CLASS, 'font-mono'].join(' ')}
            />
          </label>
          <div className="grid grid-cols-2 gap-3">
            <label className="flex flex-col gap-1.5 text-sm text-text-mut">
              GitHub owner <span className="text-text-dim">(optional)</span>
              <input
                value={owner}
                onChange={(event) => setOwner(event.target.value)}
                className={INPUT_CLASS}
              />
            </label>
            <label className="flex flex-col gap-1.5 text-sm text-text-mut">
              GitHub repo <span className="text-text-dim">(optional)</span>
              <input
                value={repo}
                onChange={(event) => setRepo(event.target.value)}
                className={INPUT_CLASS}
              />
            </label>
          </div>

          {error && (
            <p role="alert" className="text-sm text-st-attention">
              {error}
            </p>
          )}

          <div className="flex justify-end gap-2 pt-2">
            <Button type="button" variant="ghost" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={submitting}>
              {submitting ? 'Registering…' : 'Register'}
            </Button>
          </div>
        </form>
      </div>
    </div>
  )
}
