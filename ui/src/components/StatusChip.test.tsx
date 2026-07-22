import { describe, expect, it } from 'vitest'
import { render } from '@testing-library/react'
import { StatusChip } from './StatusChip'
import { STATUS_ORDER } from '../lib/status'

describe('StatusChip', () => {
  it.each(STATUS_ORDER)('renders the %s status with a count', (status) => {
    const { container } = render(<StatusChip status={status} count={3} />)
    expect(container.firstChild).toMatchSnapshot()
  })
})
