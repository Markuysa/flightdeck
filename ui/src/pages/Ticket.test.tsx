import { cleanup, fireEvent, render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import { ApiError } from '../lib/api'
import type { TicketDetail } from '../lib/types'
import { Ticket } from './Ticket'

// Partial mock: only stub the request function, keep the real ApiError class
// (a full `vi.mock('../lib/api')` automock replaces ApiError's constructor
// too, which breaks `instanceof ApiError`/`.status` — the exact thing the
// not-found/409 tests below need to be real).
vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return { ...actual, getTicket: vi.fn(), dispatchTicket: vi.fn(), approveTicket: vi.fn() }
})

// @testing-library/react's auto-cleanup only registers itself against a
// global `afterEach` (test.globals is off in vite.config.ts), so without
// this, DOM from earlier tests in this file stays mounted and later
// getBy/findBy queries see duplicates.
afterEach(cleanup)

const mockedApi = vi.mocked(api)

function detail(overrides: Partial<TicketDetail> = {}): TicketDetail {
  return {
    id: 11,
    title: 'Frontend — Ticket detail screen',
    role: 'frontend',
    depends: [2],
    body: '## Acceptance criteria\n- [ ] Renders body and criteria\n- [x] Reuses DependencyTrail',
    handoff: '',
    status: 'ready',
    branch: 'claude/011-frontend-ticket',
    pr: null,
    depends_detail: [
      {
        id: 2,
        title: 'UI scaffold',
        role: 'designer',
        depends: [],
        body: '',
        handoff: 'Tokens live in tokens.css; import, never rebuild.',
        status: 'done',
        branch: 'claude/002-ui-scaffold',
        pr: null,
      },
    ],
    ...overrides,
  }
}

function renderTicket(path = '/p/flightdeck/t/11') {
  return render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/p/:id/t/:tid" element={<Ticket />} />
        <Route path="/p/:id" element={<div>Board page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Ticket', () => {
  beforeEach(() => {
    // resetAllMocks (not clearAllMocks): clearAllMocks leaves queued
    // mockResolvedValueOnce() values in place, which then leak into the
    // next test's calls.
    vi.resetAllMocks()
  })

  it('shows a loading state before the ticket resolves', () => {
    mockedApi.getTicket.mockImplementation(() => new Promise(() => {}))
    renderTicket()

    expect(screen.getByText('Loading ticket…')).toBeInTheDocument()
  })

  it('renders the body/acceptance criteria, dependency trail, and upstream handoffs', async () => {
    mockedApi.getTicket.mockResolvedValue(detail())
    renderTicket()

    expect(await screen.findByText('Renders body and criteria')).toBeInTheDocument()
    expect(screen.getByText('Reuses DependencyTrail')).toHaveClass('line-through')

    // Dependency trail: one dot for depends=[2], coloured by depends_detail[0].status.
    const dots = screen.getAllByRole('img', { name: 'Done' })
    expect(dots.length).toBeGreaterThan(0)

    // Upstream handoff text from depends_detail[0].handoff.
    expect(screen.getByText('Tokens live in tokens.css; import, never rebuild.')).toBeInTheDocument()
    expect(screen.getByText('UI scaffold')).toBeInTheDocument()

    expect(mockedApi.getTicket).toHaveBeenCalledWith('flightdeck', 11)
  })

  it('shows the PR badge with CI colour only when the ticket is in review', async () => {
    mockedApi.getTicket.mockResolvedValueOnce(detail({ status: 'ready', pr: null }))
    const { unmount } = renderTicket()
    await screen.findByText('Renders body and criteria')
    expect(screen.queryByRole('link', { name: /#42/ })).not.toBeInTheDocument()
    unmount()

    mockedApi.getTicket.mockResolvedValueOnce(
      detail({ status: 'in_review', pr: { number: 42, url: 'https://github.com/acme/repo/pull/42', ci: 'red' } }),
    )
    renderTicket()

    const link = await screen.findByRole('link', { name: '#42' })
    expect(link).toHaveAttribute('href', 'https://github.com/acme/repo/pull/42')
    const dot = screen.getByRole('img', { name: 'CI Failing' })
    expect(dot).toHaveStyle({ backgroundColor: 'var(--st-attention)' })
  })

  it('enables Dispatch only when ready, and shows Approve merge only when in review', async () => {
    mockedApi.getTicket.mockResolvedValueOnce(detail({ status: 'ready' }))
    const { unmount } = renderTicket()
    await screen.findByText('Renders body and criteria')
    expect(screen.getByRole('button', { name: 'Dispatch' })).toBeEnabled()
    expect(screen.queryByRole('button', { name: 'Approve merge' })).not.toBeInTheDocument()
    unmount()

    mockedApi.getTicket.mockResolvedValueOnce(detail({ status: 'blocked' }))
    const second = renderTicket()
    await screen.findByText('Renders body and criteria')
    expect(screen.getByRole('button', { name: 'Dispatch' })).toBeDisabled()
    second.unmount()

    mockedApi.getTicket.mockResolvedValueOnce(
      detail({ status: 'in_review', pr: { number: 5, url: 'https://x/5', ci: 'green' } }),
    )
    renderTicket()
    await screen.findByText('Renders body and criteria')
    expect(screen.getByRole('button', { name: 'Dispatch' })).toBeDisabled()
    expect(screen.getByRole('button', { name: 'Approve merge' })).toBeEnabled()
  })

  it('dispatches and surfaces the returned session URL', async () => {
    mockedApi.getTicket.mockResolvedValue(detail({ status: 'ready' }))
    mockedApi.dispatchTicket.mockResolvedValue({ session_url: 'https://routines.example/sessions/1' })
    renderTicket()

    await screen.findByText('Renders body and criteria')
    fireEvent.click(screen.getByRole('button', { name: 'Dispatch' }))

    expect(
      await screen.findByRole('link', { name: 'https://routines.example/sessions/1' }),
    ).toBeInTheDocument()
    expect(mockedApi.dispatchTicket).toHaveBeenCalledWith('flightdeck', 11)
  })

  it('surfaces the API error message when dispatch is rejected (409 not ready)', async () => {
    mockedApi.getTicket.mockResolvedValue(detail({ status: 'ready' }))
    mockedApi.dispatchTicket.mockRejectedValue(new ApiError(409, 'ticket 11 is not ready'))
    renderTicket()

    await screen.findByText('Renders body and criteria')
    fireEvent.click(screen.getByRole('button', { name: 'Dispatch' }))

    expect(await screen.findByRole('alert')).toHaveTextContent('ticket 11 is not ready')
  })

  it('shows a not-found state for a 404', async () => {
    mockedApi.getTicket.mockRejectedValue(new ApiError(404, 'ticket 999 not found'))
    renderTicket('/p/flightdeck/t/999')

    expect(await screen.findByText('Ticket not found')).toBeInTheDocument()
  })

  it('shows an error state with a retry action on a non-404 failure', async () => {
    mockedApi.getTicket.mockRejectedValueOnce(new Error('failed to load ticket'))
    mockedApi.getTicket.mockResolvedValueOnce(detail())
    renderTicket()

    expect(await screen.findByText('failed to load ticket')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))

    expect(await screen.findByText('Renders body and criteria')).toBeInTheDocument()
  })

  it('links back to the project board', async () => {
    mockedApi.getTicket.mockResolvedValue(detail())
    renderTicket()

    fireEvent.click(await screen.findByRole('link', { name: /board/i }))
    expect(await screen.findByText('Board page')).toBeInTheDocument()
  })
})
