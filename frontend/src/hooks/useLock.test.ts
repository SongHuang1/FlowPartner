import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { useLock, LockProvider } from '@/hooks/useLock'

const mockGetLockStatus = vi.fn()
const mockUnlock = vi.fn()
const mockLock = vi.fn()

vi.mock('@/lib/api', () => ({
  getLockStatus: () => mockGetLockStatus(),
  unlock: (password: string) => mockUnlock(password),
  lock: () => mockLock(),
}))

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({
    settings: {},
    loading: false,
    error: null,
    updateSettings: vi.fn(),
    getCurrentSettings: vi.fn(),
    refreshSettings: vi.fn(),
  }),
}))

function lockWrapper({ children }: { children: ReactNode }) {
  return createElement(LockProvider, null, children)
}

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

  it('returns initial lock status', async () => {
    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.lockStatus.failed_attempts).toBe(0)
    expect(result.current.lockStatus.has_api_key).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('unlock calls API and refreshes status', async () => {
    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

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

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.unlock('TestPass123')
    })

    expect(result.current.lockStatus.locked).toBe(false)
    expect(result.current.error).toBeNull()
  })

  it('unlock sets error on failure', async () => {
    mockUnlock.mockRejectedValue(new Error('Wrong password'))

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      try {
        await result.current.unlock('WrongPass123')
      } catch {
        // Expected to throw
      }
    })

    expect(result.current.error).toBe('Wrong password')
  })

  it('unlock refreshes status on failure', async () => {
    mockUnlock.mockRejectedValue(new Error('Wrong password'))
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 1,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      try {
        await result.current.unlock('WrongPass123')
      } catch {
        // Expected to throw
      }
    })

    expect(result.current.lockStatus.failed_attempts).toBe(1)
  })

  it('lock calls API and refreshes status', async () => {
    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

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

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('lock sets error on failure', async () => {
    mockLock.mockRejectedValue(new Error('Lock failed'))

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.error).toBe('Lock failed')
  })

  it('refreshStatus updates lock status', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: false,
      failed_attempts: 0,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.refreshStatus()
    })

    expect(result.current.lockStatus.locked).toBe(false)
    expect(result.current.lockStatus.has_api_key).toBe(true)
    expect(result.current.error).toBeNull()
  })

  it('refreshStatus sets error on failure', async () => {
    mockGetLockStatus.mockRejectedValue(new Error('Network error'))

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.refreshStatus()
    })

    expect(result.current.error).toBe('Network error')
  })

  it('refreshStatus sets default error for non-Error exception', async () => {
    mockGetLockStatus.mockRejectedValue('string error')

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.refreshStatus()
    })

    expect(result.current.error).toBe('获取锁定状态失败')
  })

  it('unlock sets loading state during operation', async () => {
    let resolveUnlock: (value: unknown) => void
    mockUnlock.mockImplementation(() => new Promise((resolve) => { resolveUnlock = resolve }))

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

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

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

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

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

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

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.lock()
    })

    expect(result.current.error).toBe('锁定失败')
  })

  it('handles rate limit status', async () => {
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      locked_until: '2026-07-23T12:00:00Z',
      failed_attempts: 5,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      await result.current.refreshStatus()
    })

    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.lockStatus.locked_until).toBe('2026-07-23T12:00:00Z')
    expect(result.current.lockStatus.failed_attempts).toBe(5)
  })

  it('clears error on successful unlock after failure', async () => {
    mockUnlock.mockRejectedValueOnce(new Error('Wrong password'))
    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      try {
        await result.current.unlock('WrongPass123')
      } catch {
        // Expected
      }
    })

    expect(result.current.error).toBe('Wrong password')

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

    renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    expect(mockLock).toHaveBeenCalled()
  })

  it('updates lock status after system-lock event', async () => {
    mockLock.mockResolvedValue(undefined)
    mockGetLockStatus.mockResolvedValue({
      locked: true,
      failed_attempts: 0,
      has_api_key: true,
    })

    const { result } = renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    expect(result.current.lockStatus.locked).toBe(true)
    expect(result.current.lockStatus.has_api_key).toBe(true)
  })

  it('does not call lock() for other custom events', async () => {
    renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      window.dispatchEvent(new CustomEvent('some-other-event'))
    })

    expect(mockLock).not.toHaveBeenCalled()
  })

  it('cleans up event listener on unmount', async () => {
    const { unmount } = renderHook(() => useLock(), { wrapper: lockWrapper })

    unmount()

    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    expect(mockLock).not.toHaveBeenCalled()
  })

  it('handles lock() failure gracefully on system-lock event', async () => {
    mockLock.mockRejectedValue(new Error('Lock failed'))

    renderHook(() => useLock(), { wrapper: lockWrapper })

    await act(async () => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })

    expect(mockLock).toHaveBeenCalled()
  })
})
