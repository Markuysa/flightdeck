import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from './lib/api'
import { ApiError } from './lib/api'
import { App } from './App'

// Partial mock: only stub the probe/session functions, keep the real
// ApiError class (a full `vi.mock('./lib/api')` automock replaces
// ApiError's constructor too, which breaks `instanceof ApiError`/`.status`
// -- the exact thing the auth gate uses to tell "no session" apart from
// "backend hiccup").
vi.mock('./lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./lib/api')>()
  return { ...actual, listProjects: vi.fn(), createSession: vi.fn() }
})

afterEach(cleanup)

const mockedApi = vi.mocked(api)

function renderApp() {
  return render(
    <MemoryRouter>
      <App />
    </MemoryRouter>,
  )
}

describe('App auth gate', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('shows the Login screen, not the app, when the auth probe returns a 401', async () => {
    mockedApi.listProjects.mockRejectedValue(new ApiError(401, 'no session'))
    renderApp()

    expect(await screen.findByRole('button', { name: 'Sign in' })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: /register project/i })).not.toBeInTheDocument()
  })

  it('shows the routed app, not Login, when the auth probe succeeds', async () => {
    mockedApi.listProjects.mockResolvedValue([])
    renderApp()

    expect(await screen.findByRole('button', { name: /register project/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Sign in' })).not.toBeInTheDocument()
  })

  it('does not trap the user on Login when the probe fails with a non-401 error', async () => {
    mockedApi.listProjects
      .mockRejectedValueOnce(new ApiError(500, 'database is warming up'))
      .mockResolvedValueOnce([])
    renderApp()

    expect(await screen.findByRole('button', { name: /register project/i })).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: 'Sign in' })).not.toBeInTheDocument()
  })

  it('reveals the app after signing in with an accepted token', async () => {
    mockedApi.listProjects.mockRejectedValueOnce(new ApiError(401, 'no session'))
    mockedApi.createSession.mockResolvedValue(undefined)
    mockedApi.listProjects.mockResolvedValueOnce([])
    renderApp()

    fireEvent.change(await screen.findByLabelText('Token'), { target: { value: 'good-token' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(mockedApi.createSession).toHaveBeenCalledWith('good-token'))
    expect(await screen.findByRole('button', { name: /register project/i })).toBeInTheDocument()
  })
})
