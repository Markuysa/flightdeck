import type { DerivedStatus } from './types'

// Display metadata for each derived status, keyed by the exact string the Go
// backend emits (internal/core.DerivedStatus). The colour var is looked up at
// render time (var(--st-...)) so tailwind.config.ts / tokens.css stay the one
// place the actual colour is written (docs/DESIGN.md §2.2).
export const STATUS_ORDER: DerivedStatus[] = [
  'ready',
  'in_progress',
  'in_review',
  'needs_attention',
  'blocked',
  'done',
]

export const STATUS_META: Record<DerivedStatus, { label: string; colorVar: string }> = {
  ready: { label: 'Ready', colorVar: '--st-ready' },
  in_progress: { label: 'In progress', colorVar: '--st-progress' },
  in_review: { label: 'In review', colorVar: '--st-review' },
  needs_attention: { label: 'Needs attention', colorVar: '--st-attention' },
  blocked: { label: 'Blocked', colorVar: '--st-blocked' },
  done: { label: 'Done', colorVar: '--st-done' },
}
