import { useEffect, useRef, useState } from 'react'

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
export function useFlightDeckEvents(onEvent: (event: FlightDeckEvent) => void): boolean {
  const handlerRef = useRef(onEvent)
  const [connected, setConnected] = useState(false)

  useEffect(() => {
    handlerRef.current = onEvent
  }, [onEvent])

  useEffect(() => {
    // EventSource is a browser API that is not guaranteed to exist — an older
    // browser, a non-DOM renderer, a test environment. Without this guard the
    // app shell throws during mount and takes the whole screen with it, which
    // is a catastrophic failure mode for a feature whose entire job is a
    // convenience (auto-refresh). No stream simply means "not live".
    // No state to set: `connected` already starts false, so the absence of a
    // stream needs no announcement — just no subscription.
    if (typeof EventSource === 'undefined') return

    const source = new EventSource('/api/events', { withCredentials: true })

    // The board updates itself only while this stream is open. When it drops,
    // the screen keeps showing whatever it last fetched and looks identical to
    // a screen that is up to date — so the connection state has to be visible,
    // not merely handled.
    source.onopen = () => setConnected(true)
    source.onerror = () => setConnected(false)

    const listeners = EVENT_KINDS.map((kind) => {
      const listener = (ev: MessageEvent<string>) => {
        setConnected(true)
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

  return connected
}
