import { cleanup, render, screen } from '@testing-library/react'
import { afterEach, describe, expect, it } from 'vitest'
import { DependencyTrail } from './DependencyTrail'

afterEach(cleanup)

describe('DependencyTrail', () => {
  it('renders nothing when there are no dependencies', () => {
    const { container } = render(<DependencyTrail depends={[]} statuses={{}} />)
    expect(container.firstChild).toBeNull()
  })

  it('renders one dot per depend, coloured by its status, falling back to blocked when unknown', () => {
    render(<DependencyTrail depends={[1, 2, 3]} statuses={{ 1: 'ready', 2: 'done' }} />)

    const dots = screen.getAllByRole('img')
    expect(dots).toHaveLength(3)
    expect(dots[0]).toHaveAccessibleName('Ready')
    expect(dots[1]).toHaveAccessibleName('Done')
    expect(dots[2]).toHaveAccessibleName('Blocked')
  })
})
