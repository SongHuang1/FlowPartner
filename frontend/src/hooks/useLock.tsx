import { createContext, useContext, useState, useCallback, useEffect } from 'react'
import type { LockStatus } from '@/types'
import { getLockStatus, unlock as apiUnlock, lock as apiLock } from '@/lib/api'
import { useSettings } from '@/hooks/useSettings'

interface UseLockReturn {
  lockStatus: LockStatus
  loading: boolean
  error: string | null
  unlock: (password: string) => Promise<void>
  lock: () => Promise<void>
  refreshStatus: () => Promise<void>
}

const LockContext = createContext<UseLockReturn | null>(null)

export function LockProvider({ children }: { children: React.ReactNode }) {
  const [lockStatus, setLockStatus] = useState<LockStatus>({
    locked: true,
    failed_attempts: 0,
    has_api_key: false,
  })
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const { refreshSettings } = useSettings()

  const refreshStatus = useCallback(async (clearError = true) => {
    try {
      const status = await getLockStatus()
      setLockStatus(status)
      if (clearError) setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '获取锁定状态失败')
    } finally {
      setLoading(false)
    }
  }, [])

  const unlock = useCallback(async (password: string) => {
    setLoading(true)
    setError(null)
    try {
      await apiUnlock(password)
      await refreshSettings()
      await refreshStatus()
    } catch (e) {
      const msg = e instanceof Error ? e.message : '解锁失败'
      setError(msg)
      await refreshStatus(false)
      throw e
    } finally {
      setLoading(false)
    }
  }, [refreshStatus, refreshSettings])

  const lock = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      await apiLock()
      await refreshStatus()
    } catch (e) {
      setError(e instanceof Error ? e.message : '锁定失败')
    } finally {
      setLoading(false)
    }
  }, [refreshStatus])

  useEffect(() => {
    ;(async () => {
      await refreshStatus()
    })()
  }, [refreshStatus])

  useEffect(() => {
    const handleSystemLock = () => {
      lock().catch(() => {})
    }
    window.addEventListener('system-lock', handleSystemLock)
    return () => window.removeEventListener('system-lock', handleSystemLock)
  }, [lock])

  useEffect(() => {
    if (window.flowPartner?.onSystemFocus) {
      window.flowPartner.onSystemFocus(() => {
        refreshStatus().catch(() => {})
      })
    }
  }, [refreshStatus])

  return (
    <LockContext.Provider value={{ lockStatus, loading, error, unlock, lock, refreshStatus }}>
      {children}
    </LockContext.Provider>
  )
}

export function useLock(): UseLockReturn {
  const ctx = useContext(LockContext)
  if (!ctx) {
    throw new Error('useLock must be used within LockProvider')
  }
  return ctx
}
