import { CheckSquare, Square } from 'lucide-react'
import type { ReactNode } from 'react'

// A small, purpose-built renderer for ticket bodies (docs/tickets/*.md):
// `##` headings, `- [ ]`/`- [x]` acceptance-criteria checklists, plain `-`
// bullets, and paragraphs. Deliberately not a general markdown engine (no
// links/emphasis/tables/code) — the ticket body format doesn't need one, and
// this keeps ui/'s dependency tree unchanged (docs/tickets/011's acceptance
// criteria: "do NOT add a heavy markdown dependency").

type Block =
  | { kind: 'heading'; text: string }
  | { kind: 'checklist'; checked: boolean; text: string }
  | { kind: 'bullet'; text: string }
  | { kind: 'paragraph'; text: string }

const HEADING_RE = /^#{1,6}\s+(.*)$/
const CHECKLIST_RE = /^[-*]\s+\[([ xX])\]\s+(.*)$/
const BULLET_RE = /^[-*]\s+(.*)$/

function parseBlocks(body: string): Block[] {
  const blocks: Block[] = []
  let paragraph: string[] = []

  function flushParagraph() {
    if (paragraph.length > 0) {
      blocks.push({ kind: 'paragraph', text: paragraph.join(' ') })
      paragraph = []
    }
  }

  for (const rawLine of body.split('\n')) {
    const line = rawLine.trim()
    if (line === '') {
      flushParagraph()
      continue
    }

    const heading = line.match(HEADING_RE)
    if (heading) {
      flushParagraph()
      blocks.push({ kind: 'heading', text: heading[1] })
      continue
    }

    const checklist = line.match(CHECKLIST_RE)
    if (checklist) {
      flushParagraph()
      blocks.push({ kind: 'checklist', checked: checklist[1].toLowerCase() === 'x', text: checklist[2] })
      continue
    }

    const bullet = line.match(BULLET_RE)
    if (bullet) {
      flushParagraph()
      blocks.push({ kind: 'bullet', text: bullet[1] })
      continue
    }

    paragraph.push(line)
  }
  flushParagraph()

  return blocks
}

/** Renders a ticket's markdown `body` (incl. its `## Acceptance criteria`
 * checklist) as readable, tokened HTML — no markdown dependency. */
export function renderTicketBody(body: string): ReactNode {
  const trimmed = body.trim()
  if (!trimmed) {
    return <p className="text-sm text-text-dim">No description.</p>
  }

  return (
    <div className="flex flex-col gap-3">
      {parseBlocks(trimmed).map((block, i) => {
        switch (block.kind) {
          case 'heading':
            return (
              <h3 key={i} className="font-mono text-[10.5px] uppercase tracking-wide text-text-dim">
                {block.text}
              </h3>
            )
          case 'checklist':
            return (
              <p key={i} className="flex items-start gap-2 text-sm">
                {block.checked ? (
                  <CheckSquare className="mt-0.5 h-4 w-4 shrink-0 text-st-done" aria-hidden />
                ) : (
                  <Square className="mt-0.5 h-4 w-4 shrink-0 text-text-dim" aria-hidden />
                )}
                <span className={block.checked ? 'text-text-mut line-through' : 'text-text'}>
                  {block.text}
                </span>
              </p>
            )
          case 'bullet':
            return (
              <p key={i} className="flex items-start gap-2 text-sm text-text-mut">
                <span aria-hidden className="mt-[9px] h-1 w-1 shrink-0 rounded-chip bg-text-dim" />
                <span>{block.text}</span>
              </p>
            )
          case 'paragraph':
            return (
              <p key={i} className="text-sm leading-relaxed text-text-mut">
                {block.text}
              </p>
            )
        }
      })}
    </div>
  )
}
