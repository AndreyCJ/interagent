import { EventsOn } from '../../../wailsjs/runtime/runtime'

export type EventCallback = (payload: unknown) => void

const handlers = new Map<string, Set<EventCallback>>()

export function onEvent(name: string, cb: EventCallback): void {
  if (!handlers.has(name)) {
    handlers.set(name, new Set())
    EventsOn(name, (payload: unknown) => dispatch(name, payload))
  }
  handlers.get(name)!.add(cb)
}

export function dispatch(name: string, payload: unknown): void {
  handlers.get(name)?.forEach(cb => cb(payload))
}
