import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react'
import { MemoryRouter } from 'react-router-dom'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import * as api from '../lib/api'
import type { ProjectSummary } from '../lib/types'
import { Fleet } from './Fleet'

vi.mock('../lib/api')

// @testing-library/react's auto-cleanup only registers itself against a
// global `afterEach` (test.globals is off in vite.config.ts), so without
// this, DOM from earlier tests in this file stays mounted and later
// getBy/findBy queries see duplicates.
afterEach(cleanup)

const mockedApi = vi.mocked(api)

function summary(overrides: Partial<ProjectSummary> = {}): ProjectSummary {
  return {
    id: 'flightdeck',
    name: 'FlightDeck',
    repo_path: '/repos/flightdeck',
    remote: '',
    routine_trigger_id: '',
    owner: '',
    repo: '',
    counts: {
      ready: 2,
      in_progress: 1,
      in_review: 0,
      needs_attention: 0,
      blocked: 0,
      done: 3,
    },
    autopilot: true,
    hasLiveAgent: true,
    ...overrides,
  }
}

function renderFleet() {
  return render(
    <MemoryRouter>
      <Fleet />
    </MemoryRouter>,
  )
}

describe('Fleet', () => {
  beforeEach(() => {
    // resetAllMocks (not clearAllMocks): clearAllMocks leaves queued
    // mockResolvedValueOnce() values in place, which then leak into the
    // next test's calls.
    vi.resetAllMocks()
  })

  it('renders each project with a StatusChip count per status and its autopilot state', async () => {
    mockedApi.listProjects.mockResolvedValue([summary()])
    renderFleet()

    expect(await screen.findByText('FlightDeck')).toBeInTheDocument()

    // Scoped to the project's own card: the screen also carries fleet-wide
    // totals now, so a bare getByText('2') would be ambiguous between a
    // project's count and the fleet's sum of it.
    const card = within(screen.getByRole('article'))
    expect(card.getByText('/repos/flightdeck')).toBeInTheDocument()
    // One StatusChip per STATUS_ORDER entry, each labelled and counted.
    expect(card.getByText('Ready')).toBeInTheDocument()
    expect(card.getByText('In progress')).toBeInTheDocument()
    expect(card.getByText('In review')).toBeInTheDocument()
    expect(card.getByText('Needs attention')).toBeInTheDocument()
    expect(card.getByText('Blocked')).toBeInTheDocument()
    expect(card.getByText('Done')).toBeInTheDocument()
    expect(card.getByText('2')).toBeInTheDocument()
    expect(card.getByText('3')).toBeInTheDocument()
    expect(card.getByText('On')).toBeInTheDocument()
    // hasLiveAgent adds a second "In progress" dot (the live indicator)
    // beside the static one already inside the in_progress StatusChip.
    expect(card.getAllByRole('img', { name: 'In progress' })).toHaveLength(2)
  })

  it('omits the live-agent dot when no agent is currently active', async () => {
    mockedApi.listProjects.mockResolvedValue([summary({ hasLiveAgent: false })])
    renderFleet()

    await screen.findByText('FlightDeck')
    expect(screen.getAllByRole('img', { name: 'In progress' })).toHaveLength(1)
  })

  it('shows an empty state hint when no projects are registered', async () => {
    mockedApi.listProjects.mockResolvedValue([])
    renderFleet()

    expect(await screen.findByText('No projects registered yet')).toBeInTheDocument()
    expect(screen.getByText(/register a project's local repository/i)).toBeInTheDocument()
  })

  it('shows an error state with a retry action when the list fails to load', async () => {
    mockedApi.listProjects.mockRejectedValueOnce(new Error('failed to list projects'))
    mockedApi.listProjects.mockResolvedValueOnce([summary()])
    renderFleet()

    expect(await screen.findByText('failed to list projects')).toBeInTheDocument()
    fireEvent.click(screen.getByRole('button', { name: /try again/i }))

    expect(await screen.findByText('FlightDeck')).toBeInTheDocument()
  })

  it('opens the register dialog, submits, and refreshes the list on success', async () => {
    mockedApi.listProjects.mockResolvedValueOnce([]).mockResolvedValueOnce([summary()])
    mockedApi.createProject.mockResolvedValue({
      id: 'flightdeck',
      name: 'FlightDeck',
      repo_path: '/repos/flightdeck',
      remote: '',
      routine_trigger_id: '',
      owner: '',
      repo: '',
    })
    renderFleet()

    await screen.findByText('No projects registered yet')
    fireEvent.click(screen.getByRole('button', { name: /register project/i }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.change(screen.getByLabelText(/^name$/i), { target: { value: 'FlightDeck' } })
    fireEvent.change(screen.getByLabelText(/repository path/i), {
      target: { value: '/repos/flightdeck' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^register$/i }))

    await waitFor(() =>
      expect(mockedApi.createProject).toHaveBeenCalledWith({
        name: 'FlightDeck',
        repo_path: '/repos/flightdeck',
      }),
    )
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument())
    expect(await screen.findByText('FlightDeck')).toBeInTheDocument()
  })

  it('includes token fields in createProject only when the human fills them in', async () => {
    mockedApi.listProjects.mockResolvedValueOnce([]).mockResolvedValueOnce([summary()])
    mockedApi.createProject.mockResolvedValue({
      id: 'flightdeck',
      name: 'FlightDeck',
      repo_path: '/repos/flightdeck',
      remote: '',
      routine_trigger_id: '',
      owner: '',
      repo: '',
    })
    renderFleet()

    await screen.findByText('No projects registered yet')
    fireEvent.click(screen.getByRole('button', { name: /register project/i }))
    fireEvent.change(screen.getByLabelText(/^name$/i), { target: { value: 'FlightDeck' } })
    fireEvent.change(screen.getByLabelText(/repository path/i), {
      target: { value: '/repos/flightdeck' },
    })
    fireEvent.change(screen.getByLabelText(/routine token/i), {
      target: { value: 'routine-tok-test' },
    })
    fireEvent.change(screen.getByLabelText(/github token/i), {
      target: { value: 'gh-tok-test' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^register$/i }))

    await waitFor(() =>
      expect(mockedApi.createProject).toHaveBeenCalledWith({
        name: 'FlightDeck',
        repo_path: '/repos/flightdeck',
        routine_token: 'routine-tok-test',
        github_token: 'gh-tok-test',
      }),
    )
  })

  // The routine trigger id is what makes a project dispatchable at all: a
  // project registered without it renders its board but answers 409 to
  // POST /dispatch. Dropping it silently on the way out of this form would
  // take the product's one write action down with it, so pin that it is
  // sent when filled and omitted when blank.
  it('sends routine_trigger_id when filled, and omits it when blank', async () => {
    mockedApi.listProjects.mockResolvedValue([])
    mockedApi.createProject.mockResolvedValue({
      id: 'flightdeck',
      name: 'FlightDeck',
      repo_path: '/repos/flightdeck',
      remote: '',
      routine_trigger_id: 'trg_abc123',
      owner: '',
      repo: '',
    })
    renderFleet()

    await screen.findByText('No projects registered yet')
    fireEvent.click(screen.getByRole('button', { name: /register project/i }))
    fireEvent.change(screen.getByLabelText(/^name$/i), { target: { value: 'FlightDeck' } })
    fireEvent.change(screen.getByLabelText(/repository path/i), {
      target: { value: '/repos/flightdeck' },
    })
    fireEvent.change(screen.getByLabelText(/routine trigger id/i), {
      target: { value: '  trg_abc123  ' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^register$/i }))

    await waitFor(() =>
      expect(mockedApi.createProject).toHaveBeenCalledWith({
        name: 'FlightDeck',
        repo_path: '/repos/flightdeck',
        routine_trigger_id: 'trg_abc123',
      }),
    )
  })

  it('omits routine_trigger_id when the field is left blank', async () => {
    mockedApi.listProjects.mockResolvedValue([])
    mockedApi.createProject.mockResolvedValue({
      id: 'flightdeck',
      name: 'FlightDeck',
      repo_path: '/repos/flightdeck',
      remote: '',
      routine_trigger_id: '',
      owner: '',
      repo: '',
    })
    renderFleet()

    await screen.findByText('No projects registered yet')
    fireEvent.click(screen.getByRole('button', { name: /register project/i }))
    fireEvent.change(screen.getByLabelText(/^name$/i), { target: { value: 'FlightDeck' } })
    fireEvent.change(screen.getByLabelText(/repository path/i), {
      target: { value: '/repos/flightdeck' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^register$/i }))

    await waitFor(() =>
      expect(mockedApi.createProject).toHaveBeenCalledWith({
        name: 'FlightDeck',
        repo_path: '/repos/flightdeck',
      }),
    )
  })

  it('shows the API validation error inline and keeps the dialog open', async () => {
    mockedApi.listProjects.mockResolvedValue([])
    mockedApi.createProject.mockRejectedValue(new Error('name and repo_path are required'))
    renderFleet()

    await screen.findByText('No projects registered yet')
    fireEvent.click(screen.getByRole('button', { name: /register project/i }))
    fireEvent.change(screen.getByLabelText(/^name$/i), { target: { value: 'FlightDeck' } })
    fireEvent.change(screen.getByLabelText(/repository path/i), {
      target: { value: '/repos/flightdeck' },
    })
    fireEvent.click(screen.getByRole('button', { name: /^register$/i }))

    expect(await screen.findByRole('alert')).toHaveTextContent('name and repo_path are required')
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('closes the dialog on Escape', async () => {
    mockedApi.listProjects.mockResolvedValue([])
    renderFleet()

    await screen.findByText('No projects registered yet')
    fireEvent.click(screen.getByRole('button', { name: /register project/i }))
    expect(screen.getByRole('dialog')).toBeInTheDocument()

    fireEvent.keyDown(window, { key: 'Escape' })
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })

  it('removes a project and refreshes the list', async () => {
    mockedApi.listProjects.mockResolvedValueOnce([summary()]).mockResolvedValueOnce([])
    mockedApi.deleteProject.mockResolvedValue(undefined)
    renderFleet()

    await screen.findByText('FlightDeck')
    fireEvent.click(screen.getByRole('button', { name: /remove flightdeck/i }))

    await waitFor(() => expect(mockedApi.deleteProject).toHaveBeenCalledWith('flightdeck'))
    expect(await screen.findByText('No projects registered yet')).toBeInTheDocument()
  })

  it('opens the manage-tokens dialog from a project card and loads its status', async () => {
    mockedApi.listProjects.mockResolvedValue([summary()])
    mockedApi.getSecretsStatus.mockResolvedValue({
      routine_token_set: true,
      github_token_set: false,
    })
    renderFleet()

    await screen.findByText('FlightDeck')
    fireEvent.click(screen.getByRole('button', { name: /manage tokens for flightdeck/i }))

    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(await screen.findByText('(set)')).toBeInTheDocument()
    expect(mockedApi.getSecretsStatus).toHaveBeenCalledWith('flightdeck')
  })
})
