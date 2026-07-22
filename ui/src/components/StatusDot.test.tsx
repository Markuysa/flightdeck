import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import { StatusDot } from './StatusDot'
import { STATUS_ORDER } from '../lib/status'

describe('StatusDot', () => {
  it.each(STATUS_ORDER)('renders the %s status', (status) => {
    const { container } = render(<StatusDot status={status} />)
    expect(container.firstChild).toMatchSnapshot()
  })

  it('renders the live-pulse variant', () => {
    const { container } = render(<StatusDot status="in_progress" live />)
    expect(container.firstChild).toMatchSnapshot()
  })
})
