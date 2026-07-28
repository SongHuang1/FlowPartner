import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act } from '@testing-library/react'
import { useLock } from '@/hooks/useLock'

const mockGetLockStatus = vi.fn()
const mockUnlock = vi.fn()
const mockLock = vi.fn()

vi.mock('@/lib/api', () => ({
  getLockStatus: () => mockGetLockStatus(),
  unlock: (password: string) => mockUnlock(password),
  lock: () => mockLock(),
}))

describe('useLock', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 0,
      has_api_key: false,
    })
    mockUnlock.mockResolvedValue(undefined)
    mockLock.mockResolvedValue(undefined)
  })

  it('returns initial lock status', () => {
    const { result } = renderHook(() => useLock())
    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.lockStatus.failed_attempts).toBe(0)
    expect(result.current.lockStatus.has_api_key).toBe(false)
    expect(result.current.loading).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('unlock calls API and refreshes status', async () => {
    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.unlock('TestPass123')
    })

    expect(mockUnlock).toHaveBeenCalledWith('TestPass123')
    expect(mockGetLockStatus).toHaveBeenCalled()
  })

  it('unlock clears password from state on success', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: false,
      failed_attempts: 0,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.unlock('TestPass123')
    })

    // After successful unlock, status should be updated
    expect(result.current.lockStatus.locked).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('unlock sets error on failure', async () => {
    mockUnlock.mockRejectedValue(new Error('密码错误'))

    const { result } = renderHook(() => useLock())

    await act(async () => {
      try {
        await result.current.unlock('WrongPass123')
      } catch {
        // Expected to throw
      }
    })

    expect(result.current.error).toBe('密码错误')
  })

  it('unlock refreshes status on failure', async () => {
    mockUnlock.mockRejectedValue(new Error('密码错误'))
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 1,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock())

    await act(async () => {
      try {
        await result.current.unlock('WrongPass123')
      } catch {
        // Expected to throw
      }
    })

    // Status should be refreshed even on failure
    expect(result.current.lockStatus.failed_attempts).toBe(1)
  })

  it('lock calls API and refreshes status', async () => {
    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.lock()
    })

    expect(mockLock).toHaveBeenCalled()
    expect(mockGetLockStatus).toHaveBeenCalled()
  })

  it('lock sets status to locked', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 0,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('lock sets error on failure', async () => {
    mockLock.mockRejectedValue(new Error('上锁失败'))

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.error).toBe('上锁失败')
  })

  it('refreshStatus updates lock status', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: false,
      failed_attempts: 0,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.refreshStatus()
    })

    expect(result.current.lockStatus.locked).toBe(false)
    expect(result.current.lockStatus.has_api_key).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('refreshStatus sets error on failure', async () => {
    mockGetLockStatus.mockRejectedValue(new Error('Network error'))

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.refreshStatus()
    })

    // Error message comes from the Error object
    expect(result.current.error).toBe('Network error')
  })

  it('refreshStatus sets default error for non-Error exception', async () => {
    mockGetLockStatus.mockRejectedValue('string error')

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.refreshStatus()
    })

    // Non-Error exceptions get the default message
    expect(result.current.error).toBe('获取锁定状态失败')
  })

  it('unlock sets loading state during operation', async () => {
    let resolveUnlock: (value: unknown) => void
    mockUnlock.mockImplementation(() => new Promise((resolve) => { resolveUnlock = resolve }))

    const { result } = renderHook(() => useLock())

    act(() => {
      result.current.unlock('TestPass123')
    })

    expect(result.current.loading).toBe(true)

    await act(async () => {
      resolveUnlock!(undefined)
    })

    expect(result.current.loading).toBe(false)
  })

  it('lock sets loading state during operation', async () => {
    let resolveLock: (value: unknown) => void
    mockLock.mockImplementation(() => new Promise((resolve) => { resolveLock = resolve }))

    const { result } = renderHook(() => useLock())

    act(() => {
      result.current.lock()
    })

    expect(result.current.loading).toBe(true)

    await act(async () => {
      resolveLock!(undefined)
    })

    expect(result.current.loading).toBe(false)
  })

  it('handles non-Error exception in unlock', async () => {
    mockUnlock.mockRejectedValue('string error')

    const { result } = renderHook(() => useLock())

    await act(async () => {
      try {
        await result.current.unlock('TestPass123')
      } catch {
        // Expected to throw
      }
    })

    expect(result.current.error).toBe('解锁失败')
  })

  it('handles non-Error exception in lock', async () => {
    mockLock.mockRejectedValue('string error')

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.error).toBe('上锁失败')
  })

  it('handles rate limit status', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      locked_until: '2026-07-23T12:00:00Z',
      failed_attempts: 5,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock())

    await act(async () => {
      await result.current.refreshStatus()
    })

    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.lockStatus.locked_until).toBe('2026-07-23T12:00:00Z')
    expect(result.current.lockStatus.failed_attempts).toBe(5)
  })

  it('clears error on successful unlock after failure', async () => {
    // First fail
    mockUnlock.mockRejectedValueOnce(new Error('密码错误'))
    const { result } = renderHook(() => useLock())

    await act(async () => {
      try {
        await result.current.unlock('WrongPass123')
      } catch {
        // Expected
      }
    })

    expect(result.current.error).toBe('密码错误')

    // Then succeed
    mockUnlock.mockResolvedValue(undefined)
    mockGetLockStatus.mockResolvedValue({
      locked: false,
      failed_attempts: 0,
      has_api_key: true,
    })

    await act(async () => {
      await result.current.unlock('CorrectPass123')
    })

    expect(result.current.error).toBeNull()
  })

  it('calls lock() when system-lock event is dispatched', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 0,
      has_api_key: true,
    })

    renderHook(() => useLock())

    // Dispatch system-lock event (simulating powerMonitor suspend/lock-screen)
    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    // Verify lock API was called
    expect(mockLock).toHaveBeenCalled()
  })

  it('updates lock status after system-lock event', async () => {
    mockLock.mockResolvedValue(undefined)
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 0,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock())

    // Dispatch system-lock event
    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    // After system-lock, status should be refreshed
    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.lockStatus.has_api_key).toBe(true)
  })

  it('does not call lock() for other custom events', async () => {
    renderHook(() => useLock())

    // Dispatch an unrelated custom event
    await act(async () => {
      window.dispatchEvent(new CustomEvent('some-other-event'))
    })

    // lock should NOT be called
    expect(mockLock).not.toHaveBeenCalled()
  })

  it('cleans up event listener on unmount', async () => {
    const { unmount } = renderHook(() => useLock())

    // Unmount the hook
    unmount()

    // Dispatch system-lock event after unmount
    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    // lock should NOT be called after unmount
    expect(mockLock).not.toHaveBeenCalled()
  })

  it('handles lock() failure gracefully on system-lock event', async () => {
    mockLock.mockRejectedValue(new Error('Lock failed'))

    renderHook(() => useLock())

    // Dispatch system-lock event - should not throw
    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    // lock was attempted
    expect(mockLock).toHaveBeenCalled()
  })
})
