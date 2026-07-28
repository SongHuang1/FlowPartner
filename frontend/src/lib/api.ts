import type { Settings, Conversation, Message, LockStatus, ChatResponse } from '@/types'

const BASE = '/api'
const FETCH_TIMEOUT_MS = 5000
const CHAT_TIMEOUT_MS = 35000

interface ApiResponse<T> {
  code: number
  message: string
  data: T
  timestamp: number
  request_id: string
}

async function fetchWithTimeout(url: string, options: RequestInit = {}, timeout = FETCH_TIMEOUT_MS): Promise<Response> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeout)
  try {
    const res = await fetch(url, { ...options, signal: controller.signal })
    if (!res.ok) {
      let backendMsg = ''
      try {
        const errBody: ApiResponse<unknown> = await res.json()
        backendMsg = errBody.message || ''
      } catch { /* ignore */ }
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

export async function unlock(password: string): Promise<void> {
  const res = await fetchWithTimeout(`${BASE}/unlock`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || '解锁失败')
  }
}

export async function lock(): Promise<void> {
  await fetchWithTimeout(`${BASE}/lock`, { method: 'POST' })
}

export async function getLockStatus(): Promise<LockStatus> {
  const res = await fetchWithTimeout(`${BASE}/lock_status`)
  const data: ApiResponse<LockStatus> = await res.json()
  return data.data
}

export async function sendMessage(content: string): Promise<ChatResponse> {
  const res = await fetchWithTimeout(`${BASE}/chat`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ content }),
  }, CHAT_TIMEOUT_MS)
  const data: ApiResponse<ChatResponse> = await res.json()
  return data.data
}

export async function saveApiKey(apiKey: string, password: string): Promise<void> {
  const res = await fetchWithTimeout(`${BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ api_key: apiKey, password }),
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || '保存 API Key 失败')
  }
}

export async function clearApiKey(): Promise<void> {
  const res = await fetchWithTimeout(`${BASE}/settings/clear_api_key`, {
    method: 'POST',
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || '清除 API Key 失败')
  }
}
