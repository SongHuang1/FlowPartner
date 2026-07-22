import type { Settings, Conversation, Message } from '@/types'

const BASE = '/api'
const FETCH_TIMEOUT_MS = 5000

// API 响应统一结构
interface ApiResponse<T> {
  code: number
  message: string
  data: T
  timestamp: number
  request_id: string
}

// fetchWithTimeout 封装 fetch，添加超时和错误信息解析
async function fetchWithTimeout(url: string, options: RequestInit = {}): Promise<Response> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), FETCH_TIMEOUT_MS)
  try {
    const res = await fetch(url, { ...options, signal: controller.signal })
    if (!res.ok) {
      let backendMsg = ''
      try {
        const errBody: ApiResponse<unknown> = await res.json()
        backendMsg = errBody.message || ''
      } catch { /* 忽略解析失败 */ }
      throw new Error(backendMsg || `Request failed: ${res.status}`)
    }
    return res
  } finally {
    clearTimeout(timer)
  }
}

export async function getSettings(): Promise<Settings> {
  const res = await fetchWithTimeout(`${BASE}/settings`)
  const data: ApiResponse<Settings> = await res.json()
  return data.data
}

export async function saveSettings(settings: Settings): Promise<Settings> {
  const res = await fetchWithTimeout(`${BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  const data: ApiResponse<Settings> = await res.json()
  return data.data
}

export async function getConversation(): Promise<Conversation> {
  const res = await fetchWithTimeout(`${BASE}/conversation`)
  const data: ApiResponse<Conversation> = await res.json()
  return data.data
}

export async function saveConversation(messages: Message[]): Promise<void> {
  await fetchWithTimeout(`${BASE}/conversation`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages, updated_at: Date.now() }),
  })
}
