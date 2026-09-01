import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { useSnapshot, SnapshotProvider } from '@/hooks/useSnapshot'
import type { SnapshotStatus } from '@/types'

const mockGetSnapshots = vi.fn()
const mockGetSnapshotDetail = vi.fn()
const mockSendManualSnapshot = vi.fn()
const mockSendRestore = vi.fn()
const mockSendSystemLock = vi.fn()

let capturedGlobalEventCb: ((eventType: string, payload: string) => void) | null = null
let mockSettingsDir = ''

vi.mock('@/lib/api', () => ({
  getSnapshots: () => mockGetSnapshots(),
  getSnapshotDetail: (id: string) => mockGetSnapshotDetail(id),
}))

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({
    settings: { snapshot_dir: mockSettingsDir },
    loading: false,
    error: null,
    updateSettings: vi.fn(),
    getCurrentSettings: vi.fn(),
    refreshSettings: vi.fn(),
  }),
}))

vi.mock('@/hooks/useWebSocket', () => ({
  useWsV2: (callbacks: { onGlobalEvent?: (eventType: string, payload: string) => void }) => {
    capturedGlobalEventCb = callbacks.onGlobalEvent ?? null
    return {
      triggerSnapshot: () => mockSendManualSnapshot(),
      restoreSnapshot: (snapshotId: string, deleteExtras: boolean) => mockSendRestore(snapshotId, deleteExtras),
      systemLock: () => mockSendSystemLock(),
    }
  },
}))

function snapshotWrapper({ children }: { children: ReactNode }) {
  return createElement(SnapshotProvider, null, children)
}

function makeStatus(overrides: Partial<SnapshotStatus> = {}): SnapshotStatus {
  return { phase: 'idle', count: 0, size_bytes: 0, skipped_files: 0, ...overrides }
}

describe('useSnapshot', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    capturedGlobalEventCb = null
    mockSettingsDir = ''
    mockGetSnapshots.mockResolvedValue([])
    mockGetSnapshotDetail.mockResolvedValue(null)
  })

  it('initializes with empty state', async () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.status).toBeNull()
    expect(result.current.snapshots).toEqual([])
    expect(result.current.messages).toEqual([])
  })

  it('manualSnapshot sends manual_snapshot action', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      result.current.manualSnapshot()
    })
    expect(mockSendManualSnapshot).toHaveBeenCalled()
  })

  it('restore sends restore action with delete_extras', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      result.current.restore('20260101-120000', true)
    })
    expect(mockSendRestore).toHaveBeenCalledWith('20260101-120000', true)
  })

  it('restore sends restore action without delete_extras', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      result.current.restore('20260101-120000', false)
    })
    expect(mockSendRestore).toHaveBeenCalledWith('20260101-120000', false)
  })

  it('updates status on snapshot_status event', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      capturedGlobalEventCb?.('snapshot_status', JSON.stringify(makeStatus({ phase: 'snapshotting' })))
    })
    expect(result.current.status?.phase).toBe('snapshotting')
  })

  it('appends messages on snapshot_message events', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      capturedGlobalEventCb?.('snapshot_message', JSON.stringify({ type: 'info', text: '还原完成' }))
      capturedGlobalEventCb?.('snapshot_message', JSON.stringify({ type: 'error', text: '还原失败' }))
    })
    expect(result.current.messages).toHaveLength(2)
    expect(result.current.messages[1].type).toBe('error')
  })

  it('caps message history at 5', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      for (let i = 0; i < 7; i++) {
        capturedGlobalEventCb?.('snapshot_message', JSON.stringify({ type: 'info', text: `msg-${i}` }))
      }
    })
    expect(result.current.messages).toHaveLength(5)
    expect(result.current.messages[0].text).toBe('msg-2')
  })

  it('dismissMessages clears messages', () => {
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      capturedGlobalEventCb?.('snapshot_message', JSON.stringify({ type: 'info', text: 'hello' }))
    })
    expect(result.current.messages).toHaveLength(1)
    act(() => {
      result.current.dismissMessages()
    })
    expect(result.current.messages).toHaveLength(0)
  })

  it('sends system_lock when system locks', () => {
    renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    act(() => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })
    expect(mockSendSystemLock).toHaveBeenCalled()
  })

  it('removes system-lock listener on unmount', () => {
    const { unmount } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    unmount()
    act(() => {
      window.dispatchEvent(new CustomEvent('system-lock'))
    })
    expect(mockSendSystemLock).not.toHaveBeenCalled()
  })

  it('fetches snapshot list when dir is configured', async () => {
    mockSettingsDir = 'C:\\snap'
    mockGetSnapshots.mockResolvedValue([
      { snapshot_id: '20260101-120000', file_count: 3, total_size_bytes: 100 },
    ])

    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    await waitFor(() => expect(result.current.snapshots).toHaveLength(1))
    expect(result.current.snapshots[0].snapshot_id).toBe('20260101-120000')
  })

  it('sets error when list fetch fails', async () => {
    mockSettingsDir = 'C:\\snap'
    mockGetSnapshots.mockRejectedValue(new Error('读取失败'))

    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })
    await waitFor(() => expect(result.current.error).toBe('读取失败'))
  })

  it('returns null detail and sets error when detail fetch fails', async () => {
    mockGetSnapshotDetail.mockRejectedValue(new Error('快照不存在'))
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })

    let detail: unknown = null
    await act(async () => {
      detail = await result.current.getDetail('20260101-120000')
    })
    expect(detail).toBeNull()
    expect(result.current.error).toBe('快照不存在')
  })

  it('returns detail when fetch succeeds', async () => {
    mockGetSnapshotDetail.mockResolvedValue({
      manifest: { snapshot_id: '20260101-120000' },
      protected_files: [],
    })
    const { result } = renderHook(() => useSnapshot(), { wrapper: snapshotWrapper })

    let detail: unknown = null
    await act(async () => {
      detail = await result.current.getDetail('20260101-120000')
    })
    expect((detail as { manifest: { snapshot_id: string } }).manifest.snapshot_id).toBe('20260101-120000')
  })
})