export interface Message {
  role: 'interviewer' | 'user' | 'assistant'
  text: string
  timestamp: number
}

export interface Session {
  id: string
  chatHistory: Message[]
  startedAt: number
}

export interface AgentConfig {
  id: string
  name: string
  provider: 'local' | 'openai-compatible'
  model: string
  baseUrl: string
  apiKey: string
  systemPrompt: string
  temperature: number
}

export interface AppSettings {
  theme: 'dark' | 'light' | 'transparent'
  language: string
  shortcuts: Shortcut[]
  autoStartListening: boolean
  sttModel: string
  sttLanguage: string
}

export interface Shortcut {
  id: string
  label: string
  keys: string[]
  enabled: boolean
}

export interface AudioDevice {
  id: string
  name: string
  isDefault: boolean
}

export type OverlayMode = 'click-through' | 'interactive'

export type Permission = 'microphone' | 'screen-recording' | 'accessibility'

export interface STTModelStatus {
  installed: boolean
  path: string
}
