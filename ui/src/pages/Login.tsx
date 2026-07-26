import { useState, type FormEvent } from 'react'
import { Button } from '../components/Button'
import { INPUT_CLASS } from '../components/RegisterProjectDialog'
import { ApiError, createSession } from '../lib/api'

export interface LoginProps {
  onAuthenticated: () => void
}

/**
 * The one entry moment (docs/tickets/021): trades FLIGHTDECK_TOKEN for a
 * session cookie via createSession(). Login is chrome, not status -- per
 * DESIGN.md §1 the primary action uses --accent, never a status colour. The
 * token lives in state only until this submit settles: cleared immediately
 * after, whether it succeeds (this screen then unmounts) or fails.
 */
export function Login({ onAuthenticated }: LoginProps) {
  const [token, setToken] = useState('')
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    setError(null)
    setSubmitting(true)
    try {
      await createSession(token)
      setToken('')
      onAuthenticated()
    } catch (err) {
      setToken('')
      setSubmitting(false)
      if (err instanceof ApiError && err.status === 401) {
        setError('That token was not accepted.')
      } else {
        setError(err instanceof Error ? err.message : 'Failed to sign in.')
      }
    }
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-bg p-4">
      <div className="w-full max-w-sm rounded-card border border-border-soft bg-surface p-8">
        <h1 className="font-display text-2xl font-semibold text-text">FlightDeck</h1>
        <p className="mt-1 text-sm text-text-mut">Sign in with your routine token to continue.</p>

        <form onSubmit={handleSubmit} className="mt-6 flex flex-col gap-4">
          <label className="flex flex-col gap-1.5 text-sm text-text-mut">
            Token
            <input
              type="password"
              autoComplete="off"
              autoFocus
              required
              value={token}
              onChange={(event) => setToken(event.target.value)}
              className={INPUT_CLASS}
            />
          </label>

          {error && (
            <p role="alert" className="text-sm text-st-attention">
              {error}
            </p>
          )}

          <Button type="submit" disabled={submitting} className="w-full">
            {submitting ? 'Signing in…' : 'Sign in'}
          </Button>
        </form>

        <p className="mt-6 text-xs text-text-dim">
          This is the <code className="font-mono">FLIGHTDECK_TOKEN</code> set in the server's
          environment.
        </p>
      </div>
    </div>
  )
}
