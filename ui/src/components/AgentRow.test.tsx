import { cleanup, render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, describe, expect, it } from 'vitest'
import type { AgentSession } from '../lib/types'
import { AgentRow } from './AgentRow'

afterEach(cleanup)

const NOW = new Date('2026-07-24T12:00:00Z')

function session(overrides: Partial<AgentSession> = {}): AgentSession {
  return {
    project_id: 'flightdeck',
    project_name: 'FlightDeck',
    ticket_id: 12,
    ticket_title: 'Agents live view',
    branch: 'claude/012-frontend-agents-actions',
    session_url: '',
    started_at: '2026-07-24T11:00:00Z',
    last_activity_at: '2026-07-24T11:58:00Z',
    ...overrides,
  }
}

function renderRow(overrides: Partial<AgentSession> = {}, now: Date = NOW) {
  return render(
    <MemoryRouter>
      <ul>
        <AgentRow session={session(overrides)} now={now} />
      </ul>
    </MemoryRouter>,
  )
}

describe('AgentRow', () => {
  it('links the project to its board route', () => {
    renderRow()
    expect(screen.getByRole('link', { name: 'FlightDeck' })).toHaveAttribute(
      'href',
      '/p/flightdeck',
    )
  })

  it('shows the mono ticket id and title, linked to the ticket route', () => {
    renderRow()
    expect(screen.getByText('#12')).toBeInTheDocument()
    const ticketLink = screen.getByRole('link', { name: /Agents live view/ })
    expect(ticketLink).toHaveAttribute('href', '/p/flightdeck/t/12')
  })

  it('shows the branch in mono', () => {
    renderRow()
    expect(screen.getByText('claude/012-frontend-agents-actions')).toBeInTheDocument()
  })

  it('shows the human-readable last-activity time', () => {
    renderRow()
    expect(screen.getByText('2m ago')).toBeInTheDocument()
  })

  it('pulses the live dot for a session active within the threshold', () => {
    renderRow()
    const dot = screen.getByRole('img', { name: 'In progress' })
    expect(dot.className).toContain('animate-live-pulse')
  })

  it('renders a static dot once the session has gone idle', () => {
    renderRow({ last_activity_at: '2026-07-24T11:30:00Z' })
    const dot = screen.getByRole('img', { name: 'In progress' })
    expect(dot.className).not.toContain('animate-live-pulse')
  })

  it('omits a session link when session_url is empty (v1 default)', () => {
    renderRow()
    expect(screen.queryByRole('link', { name: /session/i })).not.toBeInTheDocument()
  })

  it('links to the session when session_url is present', () => {
    renderRow({ session_url: 'https://routines.example/sessions/9' })
    expect(screen.getByRole('link', { name: /session/i })).toHaveAttribute(
      'href',
      'https://routines.example/sessions/9',
    )
  })
})
