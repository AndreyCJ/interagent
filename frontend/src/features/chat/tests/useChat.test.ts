import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Session } from '../../../common/types/api.types'

const { mockWails } = vi.hoisted(() => ({
  mockWails: {
    SendText: vi.fn(),
    GetSession: vi.fn(),
  },
}))

const handlers: Record<string, (payload: unknown) => void> = {}

vi.mock('../../../common/utils/wails', () => mockWails)
vi.mock('../../../common/utils/events', () => ({
  onEvent: (name: string, cb: (payload: unknown) => void) => {
    handlers[name] = cb
  },
}))

import { useChat } from '../useChat'

function fire(name: string, payload?: unknown) {
  handlers[name]?.(payload)
}

describe('useChat', () => {
  beforeEach(() => {
    Object.keys(handlers).forEach(k => delete handlers[k])
    vi.clearAllMocks()
  })

  it('starts with empty messages and no error', () => {
    const { messages, loading, error } = useChat()
    expect(messages.value).toEqual([])
    expect(loading.value).toBe(false)
    expect(error.value).toBeNull()
  })

  it('load restores history from session', async () => {
    const session: Session = {
      id: 's1',
      startedAt: 100,
      chatHistory: [{ role: 'assistant', text: 'hi', timestamp: 100 }],
    }
    mockWails.GetSession.mockResolvedValue(session)

    const { messages, load } = useChat()
    await load()
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].text).toBe('hi')
  })

  it('send pushes user message and calls backend', async () => {
    mockWails.SendText.mockResolvedValue(undefined)
    const { messages, send } = useChat()
    await send('What is 2+2?')
    expect(mockWails.SendText).toHaveBeenCalledWith('What is 2+2?')
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('user')
    expect(messages.value[0].text).toBe('What is 2+2?')
  })

  it('send ignores blank text', async () => {
    const { send } = useChat()
    await send('   ')
    expect(mockWails.SendText).not.toHaveBeenCalled()
  })

  it('sets loading on llm:started', () => {
    const { loading } = useChat()
    fire('llm:started', {})
    expect(loading.value).toBe(true)
  })

  it('appends assistant message on llm:response and clears loading', () => {
    const { messages, loading } = useChat()
    fire('llm:started', {})
    fire('llm:response', { text: 'answer 42' })
    expect(loading.value).toBe(false)
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('assistant')
    expect(messages.value[0].text).toBe('answer 42')
  })

  it('sets error on llm:error and clears loading', () => {
    const { loading, error } = useChat()
    fire('llm:started', {})
    fire('llm:error', { error: 'network timeout' })
    expect(loading.value).toBe(false)
    expect(error.value).toBe('network timeout')
  })

  it('clears loading on llm:cancelled', () => {
    const { loading } = useChat()
    fire('llm:started', {})
    fire('llm:cancelled', {})
    expect(loading.value).toBe(false)
  })

  it('captures SendText rejection as error', async () => {
    mockWails.SendText.mockRejectedValue(new Error('bind failed'))
    const { error, send } = useChat()
    await send('hello')
    expect(error.value).toBe('bind failed')
  })
})
