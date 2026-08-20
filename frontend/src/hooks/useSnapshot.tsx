import { createContext, useContext, useState, useEffect, useCallback, useRef } from 'react'
import type { SnapshotStatus, SnapshotMessage, SnapshotManifest, SnapshotDetail } from '@/types'
import { getSnapshots, getSnapshotDetail } from '@/lib/api'
import { useWebSocket } from '@/hooks/useWebSocket'
import { useSettings } from '@/hooks/useSettings'

export interface UseSnapshotReturn {
  status: SnapshotStatus | null
  messages: SnapshotMessage[]
  snapshots: SnapshotManifest[]
  loading: boolean
  error: string | null
  refreshList: () => Promise<void>
  manualSnapshot: () => void
  restore: (snapshotId: string, deleteExtras: boolean) => void
  getDetail: (snapshotId: string) => Promise<SnapshotDetail | null>
  dismissMessages: () => void
}

const SnapshotContext = createContext<UseSnapshotReturn | null>(null)

export function SnapshotProvider({ children }: { children: React.ReactNode }) {
  const { sendManualSnapshot, sendRestore, sendSystemLock, onSnapshotStatus, onSnapshotMessage } = useWebSocket()
  const { settings } = useSettings()

  const [status, setStatus] = useState<SnapshotStatus | null>(null)
  const [messages, setMessages] = useState<SnapshotMessage[]>([])
  const [snapshots, setSnapshots] = useState<SnapshotManifest[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const settingsRef = useRef(settings)

  useEffect(() => {
    settingsRef.current = settings
  }, [settings])

  const refreshList = useCallback(async () => {
    if (!settingsRef.current.snapshot_dir) {
      setSnapshots([])
      setLoading(false)
      return
    }
    try {
      const list = await getSnapshots()
      setSnapshots(list)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '读取快照列表失败')
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    ;(async () => {
      await refreshList()
    })()
  }, [refreshList])

  const manualSnapshot = useCallback(() => {
    sendManualSnapshot()
  }, [sendManualSnapshot])

  const restore = useCallback((snapshotId: string, deleteExtras: boolean) => {
    sendRestore(snapshotId, deleteExtras)
  }, [sendRestore])

  const getDetail = useCallback(async (snapshotId: string): Promise<SnapshotDetail | null> => {
    try {
      return await getSnapshotDetail(snapshotId)
    } catch (e) {
      setError(e instanceof Error ? e.message : '读取快照详情失败')
      return null
    }
  }, [])

  const dismissMessages = useCallback(() => {
    setMessages([])
  }, [])

  useEffect(() => {
    const unsubStatus = onSnapshotStatus((s) => setStatus(s))
    const unsubMessage = onSnapshotMessage((m) => {
      setMessages((prev) => {
        const next = [...prev, m]
        return next.slice(-5)
      })
    })
    return () => {
      unsubStatus()
      unsubMessage()
    }
  }, [onSnapshotStatus, onSnapshotMessage])

  // 系统锁屏：刷新快照（flush）以保护未保存的工作
  useEffect(() => {
    const handleSystemLock = () => {
      sendSystemLock()
    }
    window.addEventListener('system-lock', handleSystemLock)
    return () => window.removeEventListener('system-lock', handleSystemLock)
  }, [sendSystemLock])

  return (
    <SnapshotContext.Provider
      value={{ status, messages, snapshots, loading, error, refreshList, manualSnapshot, restore, getDetail, dismissMessages }}
    >
      {children}
    </SnapshotContext.Provider>
  )
}

export function useSnapshot(): UseSnapshotReturn {
  const ctx = useContext(SnapshotContext)
  if (!ctx) {
    throw new Error('useSnapshot must be used within SnapshotProvider')
  }
  return ctx
}