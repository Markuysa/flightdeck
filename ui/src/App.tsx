import { useCallback, useEffect, useState } from 'react'
import { Route, Routes } from 'react-router-dom'
import { Shell } from './components/Shell'
import { Fleet } from './pages/Fleet'
import { Board } from './pages/Board'
import { Ticket } from './pages/Ticket'
import { AgentConfig } from './pages/AgentConfig'
import { Agents } from './pages/Agents'
import { Plan } from './pages/Plan'
import { Login } from './pages/Login'
import { ApiError, listProjects } from './lib/api'

type AuthState = 'checking' | 'authed' | 'unauthed'

/**
 * Auth gate: probes with a cheap authed call (listProjects) on mount. A 401
 * means no session cookie yet -> show Login. Any OTHER failure (network
 * hiccup, 500, ...) is NOT an auth failure -- render the app anyway so its
 * own screens show their own error/empty states, rather than trapping the
 * human on the login screen because the backend had a bad moment.
 */
export function App() {
  const [auth, setAuth] = useState<AuthState>('checking')

  const checkAuth = useCallback(async () => {
    try {
      await listProjects()
      setAuth('authed')
    } catch (err) {
      setAuth(err instanceof ApiError && err.status === 401 ? 'unauthed' : 'authed')
    }
  }, [])

  // Deferred a microtask (Promise.resolve().then(...)) per the
  // set-state-in-effect lint rule -- see docs/tickets/009's handoff.
  useEffect(() => {
    Promise.resolve()
      .then(() => checkAuth())
      .catch(() => {
        // checkAuth already turns a failure into auth state.
      })
  }, [checkAuth])

  if (auth === 'checking') return <div className="min-h-screen bg-bg" />
  if (auth === 'unauthed') return <Login onAuthenticated={() => setAuth('authed')} />

  return (
    <Routes>
      <Route element={<Shell />}>
        <Route path="/" element={<Fleet />} />
        <Route path="/p/:id" element={<Board />} />
        <Route path="/p/:id/t/:tid" element={<Ticket />} />
        <Route path="/agents" element={<Agents />} />
        <Route path="/p/:id/plan" element={<Plan />} />
        <Route path="/p/:id/agents" element={<AgentConfig />} />
      </Route>
    </Routes>
  )
}
