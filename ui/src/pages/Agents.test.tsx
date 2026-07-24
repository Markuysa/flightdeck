import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import * as sse from '../lib/sse'
import type { AgentSession } from '../lib/types'
import { Agents } from './Agents'

vi.mock('../lib/api')
vi.mock('../lib/sse')

// @testing-library/react's auto-cleanup only registers itself against a
// global `afterEach` (test.globals is off in vite.config.ts), so without
// this, DOM from earlier tests in this file stays mounted and later
// getBy/findBy queries see duplicates.
afterEach(cleanup)

const mockedApi = vi.mocked(api)
const mockedSse = vi.mocked(sse)

function session(overrides: Partial<AgentSession> = {}): AgentSession {
  return {
    project_id: 'flightdeck',
    project_name: 'FlightDeck',
    ticket_id: 12,
    ticket_title: 'Agents live view',
    branch: 'claude/012-frontend-agents-actions',
    session_url: '',
    started_at: '2026-07-24T11:00:00Z',
    last_activity_at: new Date().toISOString(),
    ...overrides,
  }
}

function renderAgents() {
  return render(
    <MemoryRouter>
      <Agents />
    </MemoryRouter>,
  )
}

describe('Agents', () => {
  beforeEach(() => {
    // resetAllMocks (not clearAllMocks): clearAllMocks leaves queued
    // mockResolvedValueOnce() values in place, which then leak into the
    // next test's calls.
    vi.resetAllMocks()
  })

  it('lists sessions with their project, ticket, and branch', async () => {
    mockedApi.listAgents.mockResolvedValue([session()])
    renderAgents()

    expect(await screen.findByText('FlightDeck')).toBeInTheDocument()
    expect(screen.getByText('#12')).toBeInTheDocument()
    expect(screen.getByText('Agents live view')).toBeInTheDocument()
    expect(screen.getByText('claude/012-frontend-agents-actions')).toBeInTheDocument()
  })

  it('shows a loading state before the sessions resolve', () => {
    mockedApi.listAgents.mockImplementation(() => new Promise(() => {}))
    renderAgents()

    expect(screen.getByText('Loading agents…')).toBeInTheDocument()
  })

  it('shows an error state with a retry action when the list fails to load', async () => {
    mockedApi.listAgents.mockRejectedValueOnce(new Error('failed to load agents'))
    mockedApi.listAgents.mockResolvedValueOnce([session()])
    renderAgents()

    expect(await screen.findByText('failed to load agents')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))

    expect(await screen.findByText('FlightDeck')).toBeInTheDocument()
  })

  it('shows an empty state when no agents are working', async () => {
    mockedApi.listAgents.mockResolvedValue([])
    renderAgents()

    expect(await screen.findByText('No agents working right now')).toBeInTheDocument()
  })

  it('refetches the sessions when an SSE event fires', async () => {
    let handler: ((event: sse.FlightDeckEvent) => void) | undefined
    mockedSse.useFlightDeckEvents.mockImplementation((cb) => {
      handler = cb
    })
    mockedApi.listAgents.mockResolvedValue([session()])
    renderAgents()

    await screen.findByText('FlightDeck')
    expect(mockedApi.listAgents).toHaveBeenCalledTimes(1)

    handler?.({ kind: 'dispatch.started', data: null })

    await waitFor(() => expect(mockedApi.listAgents).toHaveBeenCalledTimes(2))
  })
})
