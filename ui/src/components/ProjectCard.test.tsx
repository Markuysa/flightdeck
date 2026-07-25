import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import { ApiError } from '../lib/api'
import type { ProjectSummary } from '../lib/types'
import { ProjectCard } from './ProjectCard'

// Partial mock: only stub setAutopilot, keep the real ApiError class (a full
// vi.mock automock would replace ApiError's constructor too, and then
// `err instanceof Error` / `.message` wouldn't carry the server's message
// through in the component under test).
vi.mock('../lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../lib/api')>()
  return { ...actual, setAutopilot: vi.fn() }
})

afterEach(cleanup)

const mockedApi = vi.mocked(api)

function summary(overrides: Partial<ProjectSummary> = {}): ProjectSummary {
  return {
    id: 'flightdeck',
    name: 'FlightDeck',
    repo_path: '/repos/flightdeck',
    remote: '',
    owner: '',
    repo: '',
    counts: {
      ready: 0,
      in_progress: 0,
      in_review: 0,
      needs_attention: 0,
      blocked: 0,
      done: 0,
    },
    autopilot: false,
    hasLiveAgent: false,
    ...overrides,
  }
}

function renderCard(overrides: Partial<ProjectSummary> = {}) {
  return render(
    <MemoryRouter>
      <ProjectCard project={summary(overrides)} onRemove={() => {}} />
    </MemoryRouter>,
  )
}

describe('ProjectCard autopilot toggle', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('renders the current autopilot state as a labelled, keyboard-accessible switch', () => {
    renderCard({ autopilot: true })

    const toggle = screen.getByRole('switch', { name: 'Autopilot for FlightDeck' })
    expect(toggle).toHaveAttribute('aria-checked', 'true')
    expect(screen.getByText('On')).toBeInTheDocument()
  })

  it('flips the toggle, calls setAutopilot with the flipped value, and reflects the result', async () => {
    mockedApi.setAutopilot.mockResolvedValue({ on: true })
    renderCard({ autopilot: false })

    const toggle = screen.getByRole('switch', { name: 'Autopilot for FlightDeck' })
    fireEvent.click(toggle)

    expect(mockedApi.setAutopilot).toHaveBeenCalledWith('flightdeck', true)
    await waitFor(() => expect(toggle).toHaveAttribute('aria-checked', 'true'))
    expect(screen.getByText('On')).toBeInTheDocument()
  })

  it('disables the toggle while the request is in flight', async () => {
    let resolveRequest!: (value: { on: boolean }) => void
    mockedApi.setAutopilot.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = resolve
      }),
    )
    renderCard({ autopilot: false })

    const toggle = screen.getByRole('switch', { name: 'Autopilot for FlightDeck' })
    fireEvent.click(toggle)
    expect(toggle).toBeDisabled()

    resolveRequest({ on: true })
    await waitFor(() => expect(toggle).toBeEnabled())
  })

  it('surfaces a rejected request inline and leaves the displayed state unchanged', async () => {
    mockedApi.setAutopilot.mockRejectedValue(new ApiError(500, 'failed to update autopilot.json'))
    renderCard({ autopilot: false })

    const toggle = screen.getByRole('switch', { name: 'Autopilot for FlightDeck' })
    fireEvent.click(toggle)

    expect(await screen.findByRole('alert')).toHaveTextContent('failed to update autopilot.json')
    expect(toggle).toHaveAttribute('aria-checked', 'false')
    expect(toggle).toBeEnabled()
    expect(screen.getByText('Off')).toBeInTheDocument()
  })
})
