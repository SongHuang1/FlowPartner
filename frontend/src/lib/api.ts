import type { Settings, LockStatus, HistoryEntry, HistorySession, SnapshotManifest, SnapshotDetail, AgentMeta, AgentDef, AgentInput } from '@/types'

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

export async function getHistoryList(): Promise<HistoryEntry[]> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/history`)
  const data: ApiResponse<HistoryEntry[]> = await res.json()
  return data.data
}

export async function getHistorySession(sessionId: string): Promise<HistorySession> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/history/${encodeURIComponent(sessionId)}`)
  const data: ApiResponse<HistorySession> = await res.json()
  return data.data
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
    throw new Error(data.message || '解锁失败')
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

export async function saveApiKey(apiKey: string, password: string, model?: string, baseURL?: string): Promise<void> {
  await ensureReady()
  const body: Record<string, string> = { api_key: apiKey, password }
  if (model) body.model = model
  if (baseURL) body.base_url = baseURL
  const res = await fetchWithTimeout(`${BASE}/settings`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || '保存 API Key 失败')
  }
}

export async function clearApiKey(): Promise<void> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/settings/clear_api_key`, {
    method: 'POST',
  })
  const data: ApiResponse<unknown> = await res.json()
  if (data.code !== 0) {
    throw new Error(data.message || '清除 API Key 失败')
  }
}

export async function getSnapshots(): Promise<SnapshotManifest[]> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/snapshots`)
  const data: ApiResponse<{ snapshots: SnapshotManifest[] }> = await res.json()
  return data.data.snapshots || []
}

export async function getSnapshotDetail(snapshotId: string): Promise<SnapshotDetail> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/snapshots/${encodeURIComponent(snapshotId)}`)
  const data: ApiResponse<SnapshotDetail> = await res.json()
  return data.data
}

export async function listAgents(): Promise<AgentMeta[]> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/agents`)
  const data: ApiResponse<AgentMeta[]> = await res.json()
  return data.data
}

export async function getAgent(agentId: string): Promise<AgentDef> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/agents/${encodeURIComponent(agentId)}`)
  const data: ApiResponse<AgentDef> = await res.json()
  return data.data
}

export async function createAgent(input: AgentInput): Promise<AgentDef> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/agents`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  const data: ApiResponse<AgentDef> = await res.json()
  return data.data
}

export async function updateAgent(agentId: string, input: AgentInput): Promise<AgentDef> {
  await ensureReady()
  const res = await fetchWithTimeout(`${BASE}/agents/${encodeURIComponent(agentId)}`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(input),
  })
  const data: ApiResponse<AgentDef> = await res.json()
  return data.data
}

export async function deleteAgent(agentId: string): Promise<void> {
  await ensureReady()
  await fetchWithTimeout(`${BASE}/agents/${encodeURIComponent(agentId)}`, {
    method: 'DELETE',
  })
}
