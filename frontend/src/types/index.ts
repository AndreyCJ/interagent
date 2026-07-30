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
  model: string
  systemPrompt: string
  temperature: number
}

export interface AppSettings {
  theme: 'dark' | 'light' | 'transparent'
  language: string
  shortcuts: Shortcut[]
  autoStartListening: boolean
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
