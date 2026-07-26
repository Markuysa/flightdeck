import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import { ApiError } from '../lib/api'
import { Login } from './Login'

// Partial mock: only stub createSession, keep the real ApiError class (a
// full `vi.mock('../lib/api')` automock replaces ApiError's constructor
// too, which breaks `instanceof ApiError`/`.status` -- the exact thing this
// screen uses to tell a bad token apart from a degraded backend).
vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return { ...actual, createSession: vi.fn() }
})

afterEach(cleanup)

const mockedApi = vi.mocked(api)

describe('Login', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('renders a labelled password field and a Sign in button', () => {
    render(<Login onAuthenticated={vi.fn()} />)

    const input = screen.getByLabelText('Token')
    expect(input).toHaveAttribute('type', 'password')
    expect(input).toHaveAttribute('autoComplete', 'off')
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeInTheDocument()
  })

  it('submits the entered token and reveals the app on success', async () => {
    mockedApi.createSession.mockResolvedValue(undefined)
    const onAuthenticated = vi.fn()
    render(<Login onAuthenticated={onAuthenticated} />)

    fireEvent.change(screen.getByLabelText('Token'), { target: { value: 'secret-token' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    await waitFor(() => expect(mockedApi.createSession).toHaveBeenCalledWith('secret-token'))
    await waitFor(() => expect(onAuthenticated).toHaveBeenCalled())
  })

  it('disables the submit button while the request is in flight', async () => {
    let resolveSession!: () => void
    mockedApi.createSession.mockReturnValue(
      new Promise((resolve) => {
        resolveSession = resolve
      }),
    )
    const onAuthenticated = vi.fn()
    render(<Login onAuthenticated={onAuthenticated} />)

    fireEvent.change(screen.getByLabelText('Token'), { target: { value: 'secret-token' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('button', { name: /signing in/i })).toBeDisabled()
    expect(onAuthenticated).not.toHaveBeenCalled()

    resolveSession()
    await waitFor(() => expect(onAuthenticated).toHaveBeenCalled())
  })

  it('shows a friendly inline error and stays on Login when the token is rejected', async () => {
    mockedApi.createSession.mockRejectedValue(new ApiError(401, 'invalid token'))
    const onAuthenticated = vi.fn()
    render(<Login onAuthenticated={onAuthenticated} />)

    fireEvent.change(screen.getByLabelText('Token'), { target: { value: 'bad-token' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('That token was not accepted.')
    expect(onAuthenticated).not.toHaveBeenCalled()
    expect(screen.getByRole('button', { name: 'Sign in' })).toBeEnabled()
  })

  it("surfaces the server's message for a non-401 failure", async () => {
    mockedApi.createSession.mockRejectedValue(new ApiError(500, 'session store unavailable'))
    render(<Login onAuthenticated={vi.fn()} />)

    fireEvent.change(screen.getByLabelText('Token'), { target: { value: 'any-token' } })
    fireEvent.click(screen.getByRole('button', { name: 'Sign in' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('session store unavailable')
  })
})
