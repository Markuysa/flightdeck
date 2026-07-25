import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import { ApiError } from '../lib/api'
import { ManageTokensDialog } from './ManageTokensDialog'

// Partial mock: only stub the request functions, keep the real ApiError
// class (a full `vi.mock('../lib/api')` automock would replace ApiError's
// constructor too, breaking `.message` on a caught instance — see
// TicketActions.test.tsx, the pattern this file follows).
vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return { ...actual, getSecretsStatus: vi.fn(), setSecrets: vi.fn() }
})

afterEach(cleanup)

const mockedApi = vi.mocked(api)

describe('ManageTokensDialog', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('loads and shows whether each token is currently set', async () => {
    mockedApi.getSecretsStatus.mockResolvedValue({
      routine_token_set: true,
      github_token_set: false,
    })
    render(<ManageTokensDialog projectId="acme" projectName="Acme" onClose={() => {}} />)

    expect(await screen.findByText('(set)')).toBeInTheDocument()
    expect(screen.getByText('(not set)')).toBeInTheDocument()
    expect(mockedApi.getSecretsStatus).toHaveBeenCalledWith('acme')
  })

  it('submits only the filled-in fields and refreshes the status', async () => {
    mockedApi.getSecretsStatus
      .mockResolvedValueOnce({ routine_token_set: false, github_token_set: true })
      .mockResolvedValueOnce({ routine_token_set: true, github_token_set: true })
    mockedApi.setSecrets.mockResolvedValue(undefined)
    render(<ManageTokensDialog projectId="acme" projectName="Acme" onClose={() => {}} />)

    await waitFor(() => expect(mockedApi.getSecretsStatus).toHaveBeenCalledTimes(1))
    fireEvent.change(screen.getByLabelText(/routine token/i), {
      target: { value: 'routine-tok-test' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }))

    await waitFor(() =>
      expect(mockedApi.setSecrets).toHaveBeenCalledWith('acme', { routine_token: 'routine-tok-test' }),
    )
    expect(await screen.findByText('Tokens saved.')).toBeInTheDocument()
    await waitFor(() => expect(mockedApi.getSecretsStatus).toHaveBeenCalledTimes(2))
  })

  it('surfaces an API error inline and keeps the dialog open', async () => {
    mockedApi.getSecretsStatus.mockResolvedValue({
      routine_token_set: false,
      github_token_set: true,
    })
    mockedApi.setSecrets.mockRejectedValue(new ApiError(500, 'failed to set secrets'))
    render(<ManageTokensDialog projectId="acme" projectName="Acme" onClose={() => {}} />)

    await waitFor(() => expect(mockedApi.getSecretsStatus).toHaveBeenCalled())
    fireEvent.change(screen.getByLabelText(/github token/i), {
      target: { value: 'gh-tok-test' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^save$/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent('failed to set secrets')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('closes on Escape', async () => {
    mockedApi.getSecretsStatus.mockResolvedValue({
      routine_token_set: false,
      github_token_set: true,
    })
    const onClose = vi.fn()
    render(<ManageTokensDialog projectId="acme" projectName="Acme" onClose={onClose} />)

    await waitFor(() => expect(mockedApi.getSecretsStatus).toHaveBeenCalled())
    fireEvent.keyDown(window, { key: 'Escape' })
    expect(onClose).toHaveBeenCalled()
  })
})
