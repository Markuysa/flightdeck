import type { Config } from 'tailwindcss'

// Every value here reads a CSS custom property from src/styles/tokens.css —
// tailwind.config.ts never carries a literal colour/radius itself. See
// docs/DESIGN.md §2 for the source values.
export default {
  // No darkMode config: the app is dark-only (docs/DESIGN.md §1), so there is
  // no light variant to switch to.
  content: ['./index.html', './src/**/*.{ts,tsx}'],
  theme: {
    // Replaces Tailwind's default screens outright: the app has exactly the
    // two breakpoints DESIGN.md §2.4 names, nothing else. Both are min-width,
    // mobile-first — write the narrow/collapsed layout unprefixed, then
    // override with `sidebar:`/`board:` for the wider layout.
    screens: {
      sidebar: '861px', // above this width, the sidebar is a sidebar, not a top bar
      board: '1280px', // above this width, the kanban stops horizontal-scrolling
    },
    // Replaces Tailwind's default palette outright so only design tokens are
    // ever available as colour utilities (no stray `bg-red-500`).
    colors: {
      transparent: 'transparent',
      current: 'currentColor',
      bg: 'var(--bg)',
      surface: 'var(--surface)',
      'surface-2': 'var(--surface-2)',
      border: 'var(--border)',
      'border-soft': 'var(--border-soft)',
      text: 'var(--text)',
      'text-mut': 'var(--text-mut)',
      'text-dim': 'var(--text-dim)',
      accent: 'var(--accent)',
      'accent-soft': 'var(--accent-soft)',
      st: {
        ready: 'var(--st-ready)',
        progress: 'var(--st-progress)',
        review: 'var(--st-review)',
        attention: 'var(--st-attention)',
        blocked: 'var(--st-blocked)',
        done: 'var(--st-done)',
      },
    },
    extend: {
      borderRadius: {
        card: 'var(--radius-card)',
        chip: 'var(--radius-chip)',
        nested: 'var(--radius-nested)',
      },
      fontFamily: {
        display: 'var(--font-display)',
        ui: 'var(--font-ui)',
        mono: 'var(--font-mono)',
      },
      transitionDuration: {
        DEFAULT: '150ms',
      },
      // The sole standing animation in the app (§2.4): a 2s pulse on the
      // live-agent dot. prefers-reduced-motion is handled globally in
      // src/index.css, and callers should still pair this with
      // `motion-reduce:animate-none` at the use site.
      keyframes: {
        'live-pulse': {
          '0%, 100%': { opacity: '1' },
          '50%': { opacity: '0.35' },
        },
      },
      animation: {
        'live-pulse': 'live-pulse 2s ease-in-out infinite',
      },
    },
  },
  plugins: [],
} satisfies Config
