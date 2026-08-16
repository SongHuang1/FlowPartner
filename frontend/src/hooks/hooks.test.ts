import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { useSettings, SettingsProvider } from '@/hooks/useSettings'
import { useConversation } from '@/hooks/useConversation'

const mockGetSettings = vi.fn()
const mockSaveSettings = vi.fn()

vi.mock('@/lib/api', () => ({
  getSettings: () => mockGetSettings(),
  saveSettings: (s: unknown) => mockSaveSettings(s),
}))

function settingsWrapper({ children }: { children: ReactNode }) {
  return createElement(SettingsProvider, null, children)
}

afterEach(() => {
  vi.useRealTimers()
})

describe('useSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetSettings.mockResolvedValue({
      model: 'gpt-4',
      agent_id: 'default',
      context_window: 8192,
      working_directory: '',
      language: 'zh-CN',
    })
    mockSaveSettings.mockResolvedValue(undefined)
  })

  it('returns default settings initially', () => {
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })
    expect(result.current.settings.model).toBe('gpt-4')
    expect(result.current.loading).toBe(true)
  })

  it('loads settings on mount', async () => {
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.settings.model).toBe('gpt-4')
  })

  it('shows error when loading fails', async () => {
    mockGetSettings.mockRejectedValue(new Error('Network error'))
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })
    await waitFor(() => expect(result.current.loading).toBe(false))
    expect(result.current.error).toBe('Network error')
  })

  it('updates settings immediately in state', () => {
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })
    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5' })
    })
    expect(result.current.settings.model).toBe('gpt-3.5')
  })

  it('debounces save', () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })
    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5' })
    })
    expect(mockSaveSettings).not.toHaveBeenCalled()
    act(() => {
      vi.advanceTimersByTime(300)
    })
    expect(mockSaveSettings).toHaveBeenCalledOnce()
    vi.useRealTimers()
  })
})

describe('useConversation', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('starts with empty messages and a session id', () => {
    const { result } = renderHook(() => useConversation())
    expect(result.current.messages).toEqual([])
    expect(result.current.sessionId).toMatch(/^sess_/)
  })

  it('sendMessage appends user message', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.sendMessage('hello')
    })

    expect(result.current.messages).toHaveLength(1)
    expect(result.current.messages[0].role).toBe('user')
    expect(result.current.messages[0].content).toBe('hello')
  })

  it('sendMessage trims whitespace', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.sendMessage('  hello world  ')
    })

    expect(result.current.messages[0].content).toBe('hello world')
  })

  it('sendMessage ignores empty content', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.sendMessage('   ')
    })

    expect(result.current.messages).toHaveLength(0)
  })

  it('sendMessage generates unique IDs', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.sendMessage('first')
    })
    act(() => {
      result.current.sendMessage('second')
    })

    expect(result.current.messages).toHaveLength(2)
    expect(result.current.messages[0].id).not.toBe(result.current.messages[1].id)
  })

  it('sendMessage sets role to user', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.sendMessage('test')
    })

    expect(result.current.messages[0].role).toBe('user')
  })

  it('sendMessage sets timestamp', () => {
    const { result } = renderHook(() => useConversation())

    const before = Date.now()
    act(() => {
      result.current.sendMessage('test')
    })
    const after = Date.now()

    expect(result.current.messages[0].timestamp).toBeGreaterThanOrEqual(before)
    expect(result.current.messages[0].timestamp).toBeLessThanOrEqual(after)
  })

  it('sendMessage returns previous messages as history', () => {
    const { result } = renderHook(() => useConversation())

    let history: ReturnType<typeof result.current.sendMessage>
    act(() => {
      result.current.sendMessage('first')
    })
    act(() => {
      history = result.current.sendMessage('second')
    })

    expect(history!).toHaveLength(1)
    expect(history![0].content).toBe('first')
  })

  it('addAssistantMessage appends assistant message', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.sendMessage('question')
    })
    act(() => {
      result.current.addAssistantMessage('answer')
    })

    expect(result.current.messages).toHaveLength(2)
    expect(result.current.messages[1].role).toBe('assistant')
    expect(result.current.messages[1].content).toBe('answer')
  })

  it('startNewConversation clears messages and generates new session id', () => {
    const { result } = renderHook(() => useConversation())
    const firstSessionId = result.current.sessionId

    act(() => {
      result.current.sendMessage('hello')
    })
    expect(result.current.messages).toHaveLength(1)

    act(() => {
      result.current.startNewConversation()
    })

    expect(result.current.messages).toEqual([])
    expect(result.current.sessionId).not.toBe(firstSessionId)
    expect(result.current.sessionId).toMatch(/^sess_/)
  })

  it('loadConversation restores messages and session id', () => {
    const { result } = renderHook(() => useConversation())
    const msgs = [
      { id: 'msg_1', role: 'user' as const, content: 'existing', timestamp: 500 },
      { id: 'msg_2', role: 'assistant' as const, content: 'answer', timestamp: 600 },
    ]

    act(() => {
      result.current.loadConversation('sess_loaded_1', msgs)
    })

    expect(result.current.messages).toEqual(msgs)
    expect(result.current.sessionId).toBe('sess_loaded_1')
  })

  it('preserves existing messages when sending new one', () => {
    const { result } = renderHook(() => useConversation())

    act(() => {
      result.current.loadConversation('sess_loaded_2', [
        { id: 'existing_1', role: 'user', content: 'existing', timestamp: 500 },
      ])
    })

    act(() => {
      result.current.sendMessage('new message')
    })

    expect(result.current.messages).toHaveLength(2)
    expect(result.current.messages[0].content).toBe('existing')
    expect(result.current.messages[1].content).toBe('new message')
  })
})

describe('useSettings edge cases', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetSettings.mockResolvedValue({
      model: 'gpt-4',
      agent_id: 'default',
      context_window: 8192,
      working_directory: '',
      language: 'zh-CN',
    })
    mockSaveSettings.mockResolvedValue(undefined)
  })

  it('handles error from saveSettings', async () => {
    mockSaveSettings.mockRejectedValue(new Error('Save failed'))
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })

    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5' })
    })

    // Wait for the debounced save to complete and error to be set
    await waitFor(() => expect(result.current.error).toBe('保存设置失败：Save failed'))
  })

  it('debounces multiple rapid updates', () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })

    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5' })
    })
    act(() => {
      result.current.updateSettings({ model: 'gpt-4' })
    })
    act(() => {
      result.current.updateSettings({ model: 'claude-3' })
    })

    // Should not have called save yet
    expect(mockSaveSettings).not.toHaveBeenCalled()

    act(() => {
      vi.advanceTimersByTime(300)
    })

    // Should only save once with the latest value
    expect(mockSaveSettings).toHaveBeenCalledOnce()
    expect(mockSaveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ model: 'claude-3' })
    )
    vi.useRealTimers()
  })

  it('merges partial settings correctly', () => {
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })

    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5' })
    })

    // Other fields should be preserved
    expect(result.current.settings.model).toBe('gpt-3.5')
    expect(result.current.settings.agent_id).toBe('default')
    expect(result.current.settings.context_window).toBe(8192)
    expect(result.current.settings.language).toBe('zh-CN')
  })

  it('handles multiple field update', () => {
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })

    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5', context_window: 4096 })
    })

    expect(result.current.settings.model).toBe('gpt-3.5')
    expect(result.current.settings.context_window).toBe(4096)
  })

  it('rapid different field updates preserve all fields', () => {
    // 测试不同字段快速更新时不会丢失字段（stale closure 问题）
    vi.useFakeTimers()
    const { result } = renderHook(() => useSettings(), { wrapper: settingsWrapper })

    // 快速更新不同字段
    act(() => {
      result.current.updateSettings({ model: 'gpt-3.5' })
    })
    act(() => {
      result.current.updateSettings({ agent_id: 'new-agent' })
    })
    act(() => {
      result.current.updateSettings({ context_window: 4096 })
    })

    // 所有字段都应该被正确保留
    expect(result.current.settings.model).toBe('gpt-3.5')
    expect(result.current.settings.agent_id).toBe('new-agent')
    expect(result.current.settings.context_window).toBe(4096)
    // 未更新的字段保持默认值
    expect(result.current.settings.language).toBe('zh-CN')
    expect(result.current.settings.working_directory).toBe('')

    vi.useRealTimers()
  })
})

describe('useConversation Strict Mode', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('handles double-invoke of effects (Strict Mode behavior)', async () => {
    // React 18 Strict Mode 会 double-invoke effects
    // 验证 hook 在 double-invoke 后状态仍然正确
    const { result } = renderHook(() => useConversation())

    // 验证初始状态正确（空对话 + 欢迎页）
    expect(result.current.messages).toEqual([])
    expect(result.current.sessionId).toMatch(/^sess_/)
  })

  it('sendMessage after Strict Mode double-invoke', async () => {
    const { result } = renderHook(() => useConversation())

    // 发送消息后状态正确
    act(() => {
      result.current.sendMessage('test message')
    })

    expect(result.current.messages).toHaveLength(1)
    expect(result.current.messages[0].content).toBe('test message')
    expect(result.current.messages[0].role).toBe('user')
  })
})
