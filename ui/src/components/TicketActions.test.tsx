import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import { ApiError } from '../lib/api'
import { TicketActions } from './TicketActions'

// Partial mock: only stub the request functions, keep the real ApiError
// class (a full `vi.mock('../lib/api')` automock replaces ApiError's
// constructor too, which would produce instances that don't carry the
// server's message through `.message`).
vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return { ...actual, dispatchTicket: vi.fn(), approveTicket: vi.fn() }
})

afterEach(cleanup)

const mockedApi = vi.mocked(api)

describe('TicketActions', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('enables Dispatch only when the ticket is ready', () => {
    const { rerender } = render(<TicketActions projectId="p1" ticketId={1} status="ready" />)
    expect(screen.getByRole('button', { name: 'Dispatch' })).toBeEnabled()

    rerender(<TicketActions projectId="p1" ticketId={1} status="in_progress" />)
    expect(screen.getByRole('button', { name: 'Dispatch' })).toBeDisabled()

    rerender(<TicketActions projectId="p1" ticketId={1} status="blocked" />)
    expect(screen.getByRole('button', { name: 'Dispatch' })).toBeDisabled()
  })

  it('shows Approve merge only when the ticket is in review', () => {
    const { rerender } = render(<TicketActions projectId="p1" ticketId={1} status="ready" />)
    expect(screen.queryByRole('button', { name: 'Approve merge' })).not.toBeInTheDocument()

    rerender(<TicketActions projectId="p1" ticketId={1} status="in_review" />)
    expect(screen.getByRole('button', { name: 'Approve merge' })).toBeEnabled()
  })

  it('dispatches and surfaces the returned session URL', async () => {
    mockedApi.dispatchTicket.mockResolvedValue({ session_url: 'https://routines.example/sessions/9' })
    render(<TicketActions projectId="p1" ticketId={7} status="ready" />)

    fireEvent.click(screen.getByRole('button', { name: 'Dispatch' }))

    expect(await screen.findByRole('link', { name: 'https://routines.example/sessions/9' })).toHaveAttribute(
      'href',
      'https://routines.example/sessions/9',
    )
    expect(mockedApi.dispatchTicket).toHaveBeenCalledWith('p1', 7)
  })

  it('surfaces the 409 not-ready message from the API on a failed dispatch', async () => {
    mockedApi.dispatchTicket.mockRejectedValue(new ApiError(409, 'ticket 7 is not ready'))
    render(<TicketActions projectId="p1" ticketId={7} status="ready" />)

    fireEvent.click(screen.getByRole('button', { name: 'Dispatch' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('ticket 7 is not ready')
  })

  it('approves the merge and surfaces the outcome', async () => {
    mockedApi.approveTicket.mockResolvedValue(undefined)
    render(<TicketActions projectId="p1" ticketId={7} status="in_review" />)

    fireEvent.click(screen.getByRole('button', { name: 'Approve merge' }))

    expect(await screen.findByText('Merged.')).toBeInTheDocument()
    expect(mockedApi.approveTicket).toHaveBeenCalledWith('p1', 7)
  })

  it('surfaces an approve-merge failure inline', async () => {
    mockedApi.approveTicket.mockRejectedValue(new ApiError(409, 'no open PR for this ticket'))
    render(<TicketActions projectId="p1" ticketId={7} status="in_review" />)

    fireEvent.click(screen.getByRole('button', { name: 'Approve merge' }))

    await waitFor(() => expect(screen.getByRole('alert')).toHaveTextContent('no open PR for this ticket'))
  })
})
