import { beforeEach, describe, expect, it, vi } from 'vitest'

const runtimeMock = vi.hoisted(() => ({
  EventsOn: vi.fn(),
}))

vi.mock('../../../../wailsjs/runtime/runtime', () => runtimeMock)

import { dispatch, onEvent } from '../events'

describe('events', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('subscribes to the runtime once per event name', () => {
    const cb = vi.fn()
    onEvent('overlay:mode', cb)
    onEvent('overlay:mode', cb)
    expect(runtimeMock.EventsOn).toHaveBeenCalledTimes(1)
    expect(runtimeMock.EventsOn).toHaveBeenCalledWith('overlay:mode', expect.any(Function))
  })

  it('fan-outs runtime payloads to all registered callbacks', () => {
    const a = vi.fn()
    const b = vi.fn()
    onEvent('llm:response', a)
    onEvent('llm:response', b)

    const subscribe = runtimeMock.EventsOn.mock.calls[0][1] as (payload: unknown) => void
    subscribe({ text: 'hi' })

    expect(a).toHaveBeenCalledWith({ text: 'hi' })
    expect(b).toHaveBeenCalledWith({ text: 'hi' })
  })

  it('dispatch notifies registered callbacks directly', () => {
    const cb = vi.fn()
    onEvent('llm:error', cb)
    dispatch('llm:error', { error: 'boom' })
    expect(cb).toHaveBeenCalledWith({ error: 'boom' })
  })

  it('dispatch with an unregistered name is a no-op', () => {
    expect(() => dispatch('unknown:event', {})).not.toThrow()
  })
})
