import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { BoardTicket } from '../lib/types'
import { HandoffList } from './HandoffList'

afterEach(cleanup)

function dep(overrides: Partial<BoardTicket> = {}): BoardTicket {
  return {
    id: 2,
    title: 'UI scaffold',
    role: 'designer',
    depends: [],
    body: '',
    handoff: '',
    status: 'done',
    branch: 'claude/002-ui-scaffold',
    pr: null,
    ...overrides,
  }
}

describe('HandoffList', () => {
  it('renders nothing when no dependency has a handoff yet', () => {
    const { container } = render(<HandoffList dependencies={[dep({ handoff: '  ' })]} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders each dependency with a handoff: id, title, status dot, and the handoff text', () => {
    render(
      <HandoffList
        dependencies={[
          dep({ id: 2, title: 'UI scaffold', status: 'done', handoff: 'Tokens live in tokens.css.' }),
          dep({ id: 3, title: 'Still cooking', status: 'in_progress', handoff: '' }),
        ]}
      />,
    )

    expect(screen.getByText('#2')).toBeInTheDocument()
    expect(screen.getByText('UI scaffold')).toBeInTheDocument()
    expect(screen.getByText('Tokens live in tokens.css.')).toBeInTheDocument()
    expect(screen.getByRole('img', { name: 'Done' })).toBeInTheDocument()
    // #3 has no handoff text yet, so it's skipped entirely.
    expect(screen.queryByText('#3')).not.toBeInTheDocument()
  })
})
