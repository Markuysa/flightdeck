import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react'
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
    expect(screen.getByText('/repos/flightdeck')).toBeInTheDocument()
    // One StatusChip per STATUS_ORDER entry, each labelled and counted.
    expect(screen.getByText('Ready')).toBeInTheDocument()
    expect(screen.getByText('In progress')).toBeInTheDocument()
    expect(screen.getByText('In review')).toBeInTheDocument()
    expect(screen.getByText('Needs attention')).toBeInTheDocument()
    expect(screen.getByText('Blocked')).toBeInTheDocument()
    expect(screen.getByText('Done')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
    expect(screen.getByText('On')).toBeInTheDocument()
    // hasLiveAgent adds a second "In progress" dot (the live indicator)
    // beside the static one already inside the in_progress StatusChip.
    expect(screen.getAllByRole('img', { name: 'In progress' })).toHaveLength(2)
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
})
