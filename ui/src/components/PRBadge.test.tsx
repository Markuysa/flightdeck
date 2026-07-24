import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import type { PRState } from '../lib/types'
import { PRBadge } from './PRBadge'

afterEach(cleanup)

function pr(overrides: Partial<PRState> = {}): PRState {
  return { number: 42, url: 'https://github.com/acme/flightdeck/pull/42', ci: 'pending', ...overrides }
}

describe('PRBadge', () => {
  it('links the PR number (mono) to pr.url', () => {
    render(<PRBadge pr={pr()} />)
    const link = screen.getByRole('link', { name: '#42' })
    expect(link).toHaveAttribute('href', 'https://github.com/acme/flightdeck/pull/42')
    expect(link).toHaveClass('font-mono')
  })

  it.each([
    ['pending', '--st-progress', 'Pending'],
    ['green', '--st-done', 'Passing'],
    ['red', '--st-attention', 'Failing'],
    ['unknown', '--text-dim', 'Unknown'],
  ] as const)('colours CI state %s per the CI mapping', (ci, colorVar, label) => {
    render(<PRBadge pr={pr({ ci })} />)
    const dot = screen.getByRole('img', { name: `CI ${label}` })
    expect(dot).toHaveStyle({ backgroundColor: `var(${colorVar})` })
    expect(screen.getByText(label)).toBeInTheDocument()
  })
})
