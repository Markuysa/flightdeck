import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import * as sse from '../lib/sse'
import type { Board as BoardData, BoardTicket } from '../lib/types'
import { Board } from './Board'

vi.mock('../lib/api')
vi.mock('../lib/sse')

// @testing-library/react's auto-cleanup only registers itself against a
// global `afterEach` (test.globals is off in vite.config.ts), so without
// this, DOM from earlier tests in this file stays mounted and later
// getBy/findBy queries see duplicates.
afterEach(cleanup)

const mockedApi = vi.mocked(api)
const mockedSse = vi.mocked(sse)

function ticket(overrides: Partial<BoardTicket> = {}): BoardTicket {
  return {
    id: 1,
    title: 'Sample ticket',
    role: 'frontend',
    depends: [],
    body: '',
    handoff: '',
    status: 'ready',
    branch: 'ticket/1-sample',
    pr: null,
    ...overrides,
  }
}

function boardFixture(overrides: Partial<BoardData> = {}): BoardData {
  return {
    ready: [],
    in_progress: [],
    in_review: [],
    needs_attention: [],
    blocked: [],
    done: [],
    ...overrides,
  }
}

function renderBoard() {
  return render(
    <MemoryRouter initialEntries={['/p/flightdeck']}>
      <Routes>
        <Route path="/p/:id" element={<Board />} />
        <Route path="/p/:id/t/:tid" element={<div>Ticket detail page</div>} />
      </Routes>
    </MemoryRouter>,
  )
}

describe('Board', () => {
  beforeEach(() => {
    // resetAllMocks (not clearAllMocks): clearAllMocks leaves queued
    // mockResolvedValueOnce() values in place, which then leak into the
    // next test's calls.
    vi.resetAllMocks()
  })

  it('renders columns in pipeline order, each headed by a StatusChip count', async () => {
    mockedApi.getBoard.mockResolvedValue(
      boardFixture({
        ready: [ticket({ id: 1, title: 'Set up CI', role: 'backend' })],
        in_progress: [
          ticket({ id: 2, title: 'Build API', role: 'backend', status: 'in_progress', depends: [1] }),
        ],
        done: [
          ticket({ id: 4, title: 'Ship it', role: 'backend', status: 'done' }),
          ticket({ id: 5, title: 'Write docs', role: 'backend', status: 'done' }),
        ],
      }),
    )
    renderBoard()

    await screen.findByText('Set up CI')

    // getAllByText returns matches in document order, so this also proves
    // the columns render in STATUS_ORDER (pipeline order), not just that
    // each label is present somewhere on the page.
    const headings = screen.getAllByText(
      /^(Ready|In progress|In review|Needs attention|Blocked|Done)$/,
    )
    expect(headings.map((el) => el.textContent)).toEqual([
      'Ready',
      'In progress',
      'In review',
      'Needs attention',
      'Blocked',
      'Done',
    ])

    const readyColumn = headings[0].closest('section') as HTMLElement
    expect(within(readyColumn).getByText('1')).toBeInTheDocument()

    const doneColumn = headings[5].closest('section') as HTMLElement
    expect(within(doneColumn).getByText('2')).toBeInTheDocument()

    const blockedColumn = headings[4].closest('section') as HTMLElement
    expect(within(blockedColumn).getByText('0')).toBeInTheDocument()
    expect(within(blockedColumn).getByText('No tickets')).toBeInTheDocument()
  })

  it('shows a card with mono id, title, and role chip', async () => {
    mockedApi.getBoard.mockResolvedValue(
      boardFixture({ ready: [ticket({ id: 7, title: 'Wire the API client', role: 'frontend' })] }),
    )
    renderBoard()

    await screen.findByText('Wire the API client')
    expect(screen.getByText('#7')).toBeInTheDocument()
    expect(screen.getByText('frontend')).toBeInTheDocument()
  })

  it('renders a dependency dot per depend, coloured by the upstream status', async () => {
    mockedApi.getBoard.mockResolvedValue(
      boardFixture({
        ready: [ticket({ id: 1, title: 'Foundation', role: 'backend', status: 'ready' })],
        blocked: [
          ticket({
            id: 3,
            title: 'Needs foundation',
            role: 'backend',
            status: 'blocked',
            depends: [1, 99],
          }),
        ],
      }),
    )
    renderBoard()

    const card = (await screen.findByText('Needs foundation')).closest('a') as HTMLElement
    const dots = within(card).getAllByRole('img')
    expect(dots).toHaveLength(2)
    expect(dots[0]).toHaveAccessibleName('Ready')
    // #99 is not present in the board response — falls back to Blocked.
    expect(dots[1]).toHaveAccessibleName('Blocked')
  })

  it('opens the ticket route when a card is clicked', async () => {
    mockedApi.getBoard.mockResolvedValue(
      boardFixture({ ready: [ticket({ id: 9, title: 'Go somewhere', role: 'frontend' })] }),
    )
    renderBoard()

    const link = await screen.findByRole('link', { name: /Go somewhere/i })
    fireEvent.click(link)

    expect(await screen.findByText('Ticket detail page')).toBeInTheDocument()
  })

  it('refetches the board when an SSE event fires', async () => {
    let handler: ((event: sse.FlightDeckEvent) => void) | undefined
    mockedSse.useFlightDeckEvents.mockImplementation((cb) => {
      handler = cb
    })
    mockedApi.getBoard.mockResolvedValue(
      boardFixture({ ready: [ticket({ id: 1, title: 'One', role: 'backend' })] }),
    )
    renderBoard()

    await screen.findByText('One')
    expect(mockedApi.getBoard).toHaveBeenCalledTimes(1)

    handler?.({ kind: 'board.changed', data: null })

    await waitFor(() => expect(mockedApi.getBoard).toHaveBeenCalledTimes(2))
  })

  it('shows a loading state before the board resolves', () => {
    mockedApi.getBoard.mockImplementation(() => new Promise(() => {}))
    renderBoard()

    expect(screen.getByText('Loading board…')).toBeInTheDocument()
  })

  it('shows an error state with a retry action when the board fails to load', async () => {
    mockedApi.getBoard.mockRejectedValueOnce(new Error('failed to load board'))
    mockedApi.getBoard.mockResolvedValueOnce(
      boardFixture({ ready: [ticket({ id: 1, title: 'One', role: 'backend' })] }),
    )
    renderBoard()

    expect(await screen.findByText('failed to load board')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))

    expect(await screen.findByText('One')).toBeInTheDocument()
  })

  it('shows an empty state hint when the project has no tickets', async () => {
    mockedApi.getBoard.mockResolvedValue(boardFixture())
    renderBoard()

    expect(await screen.findByText('No tickets yet')).toBeInTheDocument()
  })
})
