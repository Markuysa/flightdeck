import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { renderTicketBody } from './markdown'

afterEach(cleanup)

describe('renderTicketBody', () => {
  it('renders a placeholder for an empty body', () => {
    render(<div>{renderTicketBody('  ')}</div>)
    expect(screen.getByText('No description.')).toBeInTheDocument()
  })

  it('renders headings, a checklist (checked and unchecked), bullets, and paragraphs', () => {
    render(
      <div>
        {renderTicketBody(
          [
            'Ticket detail (US-3): body, criteria, dependency trail.',
            '',
            '## Acceptance criteria',
            '- [ ] Renders body and criteria',
            '- [x] Renders the dependency trail',
            '',
            '## Likely files',
            '- `ui/src/pages/Ticket.tsx`',
          ].join('\n'),
        )}
      </div>,
    )

    expect(screen.getByText('Acceptance criteria')).toBeInTheDocument()
    expect(screen.getByText('Renders body and criteria')).toBeInTheDocument()
    expect(screen.getByText('Renders the dependency trail')).toHaveClass('line-through')
    expect(screen.getByText('`ui/src/pages/Ticket.tsx`')).toBeInTheDocument()
    expect(
      screen.getByText('Ticket detail (US-3): body, criteria, dependency trail.'),
    ).toBeInTheDocument()
  })
})
