export interface Message {
  id: string
  role: 'user' | 'assistant'
  content: string
  timestamp: number
}

export interface HistoryEntry {
  session_id: string
  title: string
  updated_at: number
  message_count: number
}

export interface HistoryMessage {
  role: 'user' | 'assistant'
  content: string
}

export interface HistorySession {
  session_id: string
  messages: HistoryMessage[]
}

export interface Settings {
  model: string
  agent_id: string
  context_window: number
  working_directory: string
  language: string

  base_url: string
  encrypted_api_key: string
  model_name: string

  system_prompt: string
  temperature: number

  close_behavior: 'minimize' | 'quit' | 'ask'
  close_remembered: boolean

  window_x: number
  window_y: number
  window_width: number
  window_height: number
  sidebar_visible: boolean
  sidebar_view: string
}

export interface LockStatus {
  locked: boolean
  locked_until?: string
  failed_attempts: number
  has_api_key: boolean
}

export interface WindowState {
  x: number
  y: number
  width: number
  height: number
  sidebar_visible: boolean
  sidebar_view: string
}
