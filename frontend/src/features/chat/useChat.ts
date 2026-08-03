import { onEvent } from '../../common/utils/events'
import { GetSession, SendText } from '../../common/utils/wails'
import { ref } from 'vue'

export interface ChatMessage {
  role: string
  text: string
  timestamp?: number
}

export function useChat() {
  const messages = ref<ChatMessage[]>([])
  const loading = ref(false)
  const error = ref<string | null>(null)

  async function load(): Promise<void> {
    try {
      const session = await GetSession()
      const history =
        (session as unknown as { ChatHistory?: ChatMessage[] }).ChatHistory ??
        (session as { chatHistory?: ChatMessage[] }).chatHistory ??
        []
      messages.value = history.map(m => ({
        role: m.role,
        text: m.text,
        timestamp: m.timestamp,
      }))
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  }

  async function send(text: string): Promise<void> {
    const trimmed = text.trim()
    if (!trimmed) return
    messages.value.push({ role: 'user', text: trimmed })
    try {
      await SendText(trimmed)
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  }

  onEvent('llm:started', () => {
    loading.value = true
    error.value = null
  })

  onEvent('llm:response', payload => {
    const p = payload as { text?: string }
    if (p?.text) {
      messages.value.push({ role: 'assistant', text: p.text })
    }
    loading.value = false
  })

  onEvent('llm:error', payload => {
    const p = payload as { error?: string }
    error.value = p?.error ?? 'LLM error'
    loading.value = false
  })

  onEvent('llm:cancelled', () => {
    loading.value = false
  })

  return { messages, loading, error, load, send }
}
