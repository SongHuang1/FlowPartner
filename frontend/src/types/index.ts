export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: number
}

export interface Conversation {
  messages: Message[]
  updated_at: number
}

export interface Settings {
  model: string
  agent_id: string
  context_window: number
  working_directory: string
  language: string
}
