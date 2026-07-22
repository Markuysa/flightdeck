import { useEffect, useRef } from 'react'

// The event kinds GET /api/events emits (docs/ARCHITECTURE.md API contract).
export const EVENT_KINDS = ['board.changed', 'dispatch.started', 'ci.changed'] as const
export type EventKind = (typeof EVENT_KINDS)[number]

export interface FlightDeckEvent {
  kind: EventKind
  data: unknown
}

function safeParse(raw: string): unknown {
  try {
    return JSON.parse(raw)
  } catch {
    return raw
  }
}

/**
 * Subscribes to GET /api/events and calls onEvent for each named SSE event.
 * The subscription is created once per mount (not re-created when onEvent's
 * identity changes) and torn down on unmount.
 */
export function useFlightDeckEvents(onEvent: (event: FlightDeckEvent) => void): void {
  const handlerRef = useRef(onEvent)
  useEffect(() => {
    handlerRef.current = onEvent
  }, [onEvent])

  useEffect(() => {
    const source = new EventSource('/api/events', { withCredentials: true })

    const listeners = EVENT_KINDS.map((kind) => {
      const listener = (ev: MessageEvent<string>) => {
        handlerRef.current({ kind, data: safeParse(ev.data) })
      }
      source.addEventListener(kind, listener)
      return { kind, listener }
    })

    return () => {
      for (const { kind, listener } of listeners) {
        source.removeEventListener(kind, listener)
      }
      source.close()
    }
  }, [])
}
