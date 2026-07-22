#!/usr/bin/env node
// Fails the build if a hex colour literal appears anywhere in ui/src/ outside
// src/styles/tokens.css, the single allowed home for design-token values
// (docs/DESIGN.md §2, CLAUDE.md "Tokens are law"). Wired into `npm run lint`.
import { readdirSync, readFileSync, statSync } from 'node:fs'
import { join, relative, extname, sep } from 'node:path'
import { fileURLToPath } from 'node:url'

const SRC_DIR = fileURLToPath(new URL('../src', import.meta.url))
const ALLOWED_FILE = join('src', 'styles', 'tokens.css')
const CHECKED_EXTENSIONS = new Set(['.ts', '.tsx', '.js', '.jsx', '.css'])
// Valid CSS hex lengths: #rgb #rgba #rrggbb #rrggbbaa
const HEX_PATTERN = /#(?:[0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})\b/g

function walk(dir, files = []) {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry)
    const info = statSync(full)
    if (info.isDirectory()) {
      walk(full, files)
    } else if (CHECKED_EXTENSIONS.has(extname(full))) {
      files.push(full)
    }
  }
  return files
}

const violations = []

for (const file of walk(SRC_DIR)) {
  const relPath = relative(join(SRC_DIR, '..'), file).split(sep).join('/')
  if (relPath === ALLOWED_FILE.split(sep).join('/')) continue

  const contents = readFileSync(file, 'utf8')
  const lines = contents.split('\n')
  lines.forEach((line, i) => {
    const matches = line.match(HEX_PATTERN)
    if (matches) {
      violations.push({ file: relPath, line: i + 1, matches, text: line.trim() })
    }
  })
}

if (violations.length > 0) {
  console.error('Hardcoded hex colour literal(s) found outside src/styles/tokens.css:\n')
  for (const v of violations) {
    console.error(`  ${v.file}:${v.line}  ${v.matches.join(', ')}`)
    console.error(`    ${v.text}`)
  }
  console.error(
    '\nEvery colour must be a CSS custom property in src/styles/tokens.css, referenced ' +
      'via var() or the Tailwind theme. See docs/DESIGN.md §2.',
  )
  process.exit(1)
}

console.log('check-tokens: no stray hex literals outside src/styles/tokens.css.')
