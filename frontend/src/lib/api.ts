import type { Settings, Conversation, Message, LockStatus } from '@/types'

const FETCH_TIMEOUT_MS = 5000

interface ApiResponse<T> {
  code: number
  message: string
  data: T
  timestamp: number
  request_id: string
}

let BASE = ''
let apiReady = false
let cachedPort: number | null = null
const readyListeners: Set<() => void> = new Set()

export function initApi(port: number): void {
  BASE = `http://localhost:${port}/api`
  cachedPort = port
  apiReady = true
  readyListeners.forEach((cb) => cb())
  readyListeners.clear()
}

export function updateApiBase(port: number): void {
  BASE = `http://localhost:${port}/api`
  cachedPort = port
}

export function getApiPort(): number | null {
  return cachedPort
}

export function getApiReady(): boolean {
  return apiReady
}

export function onApiReady(callback: () => void): () => void {
  if (apiReady) {
    callback()
    return () => {}
  }
  readyListeners.add(callback)
  return () => {
    readyListeners.delete(callback)
  }
}

async function ensureReady(): Promise<void> {
  if (apiReady) return
  return new Promise((resolve) => {
    const unsub = onApiReady(() => {
      unsub()
      resolve()
    })
  })
}

async function fetchWithTimeout(
  url: string,
  options: RequestInit = {},
  timeout = FETCH_TIMEOUT_MS,
): Promise<Response> {
  const controller = new AbortController()
  const timer = setTimeout(() => controller.abort(), timeout)
  try {
    const res = await fetch(url, { ...options, signal: controller.signal })
    if (!res.ok) {
      let backendMsg = ''
      try {
        const errBody: ApiResponse<unknown> = await res.json()
        backendMsg = errBody.message || ''
      } catch {
        /* ignore */
      }
      throw new Error(backendMsg || `Request failed: ${res.status}`)
    }
    return res
  } finally {
    clearTimeout(timer)
  }
}

export async function getSettings(): Promise<Settings> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/settings`)
  const data: ApiResponse<Settings> = await res.json()
  return data.data
}

export async function saveSettings(settings: Settings): Promise<Settings> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  const data: ApiResponse<Settings> = await res.json()
  return data.data
}

export async function getConversation(): Promise<Conversation> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/conversation`)
  const data: ApiResponse<Conversation> = await res.json()
  return data.data
}

export async function saveConversation(messages: Message[]): Promise<void> {
  await ensureReady()
  await fetchWithTimeout(`${BASE}/conversation`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ messages, updated_at: Date.now() }),
  })
}

export async function unlock(password: string): Promise<void> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/unlock`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ password }),
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || 'Unlock failed')
  }
}

export async function lock(): Promise<void> {
  await ensureReady()
  await fetchWithTimeout(`${BASE}/lock`, { method: 'POST' })
}

export async function getLockStatus(): Promise<LockStatus> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/lock_status`)
  const data: ApiResponse<LockStatus> = await res.json()
  return data.data
}

export async function saveApiKey(apiKey: string, password: string): Promise<void> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ api_key: apiKey, password }),
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || 'Failed to save API Key')
  }
}

export async function clearApiKey(): Promise<void> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/settings/clear_api_key`, {
    method: 'POST',
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || 'Failed to clear API Key')
  }
}
