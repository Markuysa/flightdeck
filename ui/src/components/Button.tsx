import type { ButtonHTMLAttributes } from 'react'

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'ghost'
}

const BASE =
  'inline-flex items-center justify-center gap-1.5 rounded-nested px-3 py-1.5 text-sm ' +
  'font-medium transition-colors duration-150 disabled:cursor-not-allowed disabled:opacity-50'

const VARIANTS: Record<NonNullable<ButtonProps['variant']>, string> = {
  primary: 'bg-accent text-bg hover:opacity-90',
  ghost: 'text-text-mut hover:bg-surface-2 hover:text-text',
}

/** The one interactive-action primitive (dispatch, approve, autopilot toggle,
 * ...): --accent for primary actions, everything else stays neutral
 * (docs/DESIGN.md §1 — status colours are the only accent). */
export function Button({ variant = 'primary', className = '', ...props }: ButtonProps) {
  return <button className={[BASE, VARIANTS[variant], className].filter(Boolean).join(' ')} {...props} />
}
