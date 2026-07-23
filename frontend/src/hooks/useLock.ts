import { useState, useCallback, useEffect } from 'react'
import type { LockStatus } from '@/types'
import { getLockStatus, unlock as apiUnlock, lock as apiLock } from '@/lib/api'

interface UseLockReturn {
  lockStatus: LockStatus
  loading: boolean
  error: string | null
  unlock: (password: string) => Promise<void>
  lock: () => Promise<void>
  refreshStatus: () => Promise<void>
}

export function useLock(): UseLockReturn {
  const [lockStatus, setLockStatus] = useState<LockStatus>({
    locked: true,
    failed_attempts: 0,
    has_api_key: false,
  })
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refreshStatus = useCallback(async (clearError = true) => {
    try {
      const status = await getLockStatus()
      setLockStatus(status)
      if (clearError) setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : '获取锁定状态失败')
    }
  }, [])

  const unlock = useCallback(async (password: string) => {
    setLoading(true)
    setError(null)
    try {
      await apiUnlock(password)
      await refreshStatus()
    } catch (e) {
      const msg = e instanceof Error ? e.message : '解锁失败'
      setError(msg)
      await refreshStatus(false)
      throw e
    } finally {
      setLoading(false)
    }
  }, [refreshStatus])

  const lock = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      await apiLock()
      await refreshStatus()
    } catch (e) {
      setError(e instanceof Error ? e.message : '上锁失败')
    } finally {
      setLoading(false)
    }
  }, [refreshStatus])

  useEffect(() => {
    const handleSystemLock = () => {
      lock().catch(() => {})
    }
    window.addEventListener('system-lock', handleSystemLock)
    return () => window.removeEventListener('system-lock', handleSystemLock)
  }, [lock])

  return { lockStatus, loading, error, unlock, lock, refreshStatus }
}
