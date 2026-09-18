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

  it('streams llm:partial into an assistant message in place', () => {
    const { messages } = useChat()
    fire('llm:started', {})
    fire('llm:partial', { text: 'Hel' })
    fire('llm:partial', { text: 'Hello' })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('assistant')
    expect(messages.value[0].text).toBe('Hello')
    expect(messages.value[0].streaming).toBe(true)
  })

  it('finalizes the streaming message on llm:response', () => {
    const { messages, loading } = useChat()
    fire('llm:started', {})
    fire('llm:partial', { text: 'part' })
    fire('llm:response', { text: 'full answer' })
    expect(loading.value).toBe(false)
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].text).toBe('full answer')
    expect(messages.value[0].streaming).toBe(false)
  })

  it('keeps the existing llm:response behavior when no partial arrived', () => {
    const { messages } = useChat()
    fire('llm:started', {})
    fire('llm:response', { text: 'direct' })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('assistant')
    expect(messages.value[0].streaming).toBeUndefined()
  })

  it('clears the streaming flag on the last assistant message on llm:error', () => {
    const { messages } = useChat()
    fire('llm:started', {})
    fire('llm:partial', { text: 'partial' })
    fire('llm:error', { error: 'network timeout' })
    expect(messages.value[messages.value.length - 1].role).toBe('assistant')
    expect(messages.value[messages.value.length - 1].streaming).toBe(false)
  })

  it('clears the streaming flag on the last assistant message on llm:cancelled', () => {
    const { messages } = useChat()
    fire('llm:started', {})
    fire('llm:partial', { text: 'partial' })
    fire('llm:cancelled', {})
    expect(messages.value[messages.value.length - 1].role).toBe('assistant')
    expect(messages.value[messages.value.length - 1].streaming).toBe(false)
  })

  it('clears a stale streaming flag before sending a new user message', async () => {
    mockWails.SendText.mockResolvedValue(undefined)
    const { messages, send } = useChat()
    fire('llm:started', {})
    fire('llm:partial', { text: 'stale' })
    await send('next question')
    const assistant = messages.value[messages.value.length - 2]
    expect(assistant.role).toBe('assistant')
    expect(assistant.streaming).toBe(false)
  })

  it('appends mic transcription as a user message on transcription:done', () => {
    const { messages } = useChat()
    fire('transcription:done', {
      text: 'Hello world',
      confidence: 0.9,
      language: 'en',
      source: 'mic',
    })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('user')
    expect(messages.value[0].text).toBe('Hello world')
  })

  it('appends system transcription as an interviewer message on transcription:done', () => {
    const { messages } = useChat()
    fire('transcription:done', {
      text: 'What is your approach?',
      confidence: 0.92,
      language: 'en',
      source: 'system',
    })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('interviewer')
    expect(messages.value[0].text).toBe('What is your approach?')
  })

  it('appends mic committed transcription as a user message on transcription:committed', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'my answer', source: 'mic' })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('user')
    expect(messages.value[0].text).toBe('my answer')
  })

  it('appends system committed transcription as an interviewer message on transcription:committed', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'the key', source: 'system' })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('interviewer')
    expect(messages.value[0].text).toBe('the key')
  })

  it('grows the interviewer message in place on cumulative transcription:committed', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'the key', source: 'system' })
    fire('transcription:committed', { text: 'the key point', source: 'system' })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('interviewer')
    expect(messages.value[0].text).toBe('the key point')
  })

  it('grows the user message in place on cumulative mic committed', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'my', source: 'mic' })
    fire('transcription:committed', { text: 'my answer', source: 'mic' })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('user')
    expect(messages.value[0].text).toBe('my answer')
  })

  it('finalizes the growing interviewer message on transcription:done', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'the key point', source: 'system' })
    fire('transcription:done', {
      text: 'the key point you made is',
      confidence: 0.9,
      language: 'en',
      source: 'system',
    })
    expect(messages.value).toHaveLength(1)
    expect(messages.value[0].role).toBe('interviewer')
    expect(messages.value[0].text).toBe('the key point you made is')
  })

  it('does not clobber an assistant answer when finalizing the interviewer phrase', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'the key', source: 'system' })
    fire('llm:started', {})
    fire('llm:response', { text: 'answer' })
    fire('transcription:committed', { text: 'the key point', source: 'system' })
    fire('transcription:done', {
      text: 'the key point you made is',
      confidence: 0.9,
      language: 'en',
      source: 'system',
    })
    expect(messages.value).toHaveLength(2)
    expect(messages.value[0].role).toBe('interviewer')
    expect(messages.value[0].text).toBe('the key point you made is')
    expect(messages.value[1].role).toBe('assistant')
    expect(messages.value[1].text).toBe('answer')
  })

  it('starts a fresh message for the next phrase after transcription:done', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'first part', source: 'system' })
    fire('transcription:done', {
      text: 'first part tail',
      confidence: 0.9,
      language: 'en',
      source: 'system',
    })
    fire('transcription:committed', { text: 'second', source: 'system' })
    expect(messages.value).toHaveLength(2)
    expect(messages.value[0].text).toBe('first part tail')
    expect(messages.value[1].role).toBe('interviewer')
    expect(messages.value[1].text).toBe('second')
  })

  it('handles the two sources independently', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: 'question', source: 'system' })
    fire('transcription:committed', { text: 'my answer', source: 'mic' })
    fire('transcription:committed', { text: 'question is', source: 'system' })
    expect(messages.value.map(m => [m.role, m.text])).toEqual([
      ['interviewer', 'question is'],
      ['user', 'my answer'],
    ])
  })

  it('ignores transcription:committed without text', () => {
    const { messages } = useChat()
    fire('transcription:committed', { text: '', source: 'system' })
    expect(messages.value).toEqual([])
  })

  it('ignores transcription:done without text', () => {
    const { messages } = useChat()
    fire('transcription:done', { text: '', source: 'system' })
    expect(messages.value).toEqual([])
  })

  it('flips transcribing on stt:processing and stt:idle', () => {
    const { transcribing } = useChat()
    expect(transcribing.value).toBe(false)
    fire('stt:processing', { source: 'system' })
    expect(transcribing.value).toBe(true)
    fire('stt:idle', { source: 'system' })
    expect(transcribing.value).toBe(false)
  })

  it('keeps transcribing while any source is busy', () => {
    const { transcribing } = useChat()
    fire('stt:processing', { source: 'system' })
    fire('stt:processing', { source: 'mic' })
    fire('stt:idle', { source: 'system' })
    expect(transcribing.value).toBe(true)
    fire('stt:idle', { source: 'mic' })
    expect(transcribing.value).toBe(false)
  })

  it('ignores stt status events without source', () => {
    const { transcribing } = useChat()
    fire('stt:processing', {})
    expect(transcribing.value).toBe(false)
  })
})
