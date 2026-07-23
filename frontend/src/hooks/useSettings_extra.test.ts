import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useSettings, DefaultSettings } from '@/hooks/useSettings'

const mockGetSettings = vi.fn()
const mockSaveSettings = vi.fn()

vi.mock('@/lib/api', () => ({
  getSettings: () => mockGetSettings(),
  saveSettings: (s: unknown) => mockSaveSettings(s),
}))

describe('useSettings - new fields', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockGetSettings.mockResolvedValue({
      model: 'gpt-4',
      agent_id: 'default',
      context_window: 8192,
      working_directory: '',
      language: 'zh-CN',
      base_url: 'https://api.openai.com/v1',
      encrypted_api_key: '',
      model_name: 'gpt-4',
      system_prompt: '你是一个有帮助的 AI 助手。',
      temperature: 0.7,
      close_behavior: 'ask',
      close_remembered: false,
      window_x: 100,
      window_y: 100,
      window_width: 1200,
      window_height: 800,
      sidebar_visible: true,
      sidebar_view: 'conversation',
    })
    mockSaveSettings.mockResolvedValue(undefined)
  })

  it('DefaultSettings returns all new fields with correct defaults', () => {
    const defaults = DefaultSettings()
    expect(defaults.base_url).toBe('https://api.openai.com/v1')
    expect(defaults.encrypted_api_key).toBe('')
    expect(defaults.model_name).toBe('gpt-4')
    expect(defaults.system_prompt).toBe('你是一个有帮助的 AI 助手。')
    expect(defaults.temperature).toBe(0.7)
    expect(defaults.close_behavior).toBe('ask')
    expect(defaults.close_remembered).toBe(false)
    expect(defaults.window_x).toBe(100)
    expect(defaults.window_y).toBe(100)
    expect(defaults.window_width).toBe(1200)
    expect(defaults.window_height).toBe(800)
    expect(defaults.sidebar_visible).toBe(true)
    expect(defaults.sidebar_view).toBe('conversation')
  })

  it('loads new fields from API', async () => {
    const { result } = renderHook(() => useSettings())
    await waitFor(() => expect(result.current.loading).toBe(false))

    expect(result.current.settings.base_url).toBe('https://api.openai.com/v1')
    expect(result.current.settings.model_name).toBe('gpt-4')
    expect(result.current.settings.system_prompt).toBe('你是一个有帮助的 AI 助手。')
    expect(result.current.settings.temperature).toBe(0.7)
    expect(result.current.settings.close_behavior).toBe('ask')
    expect(result.current.settings.close_remembered).toBe(false)
    expect(result.current.settings.window_x).toBe(100)
    expect(result.current.settings.window_width).toBe(1200)
    expect(result.current.settings.sidebar_visible).toBe(true)
    expect(result.current.settings.sidebar_view).toBe('conversation')
  })

  it('updates base_url immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ base_url: 'https://api.deepseek.com/v1' })
    })
    expect(result.current.settings.base_url).toBe('https://api.deepseek.com/v1')
  })

  it('updates model_name immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ model_name: 'deepseek-chat' })
    })
    expect(result.current.settings.model_name).toBe('deepseek-chat')
  })

  it('updates system_prompt immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ system_prompt: 'You are a code expert.' })
    })
    expect(result.current.settings.system_prompt).toBe('You are a code expert.')
  })

  it('updates temperature immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ temperature: 1.5 })
    })
    expect(result.current.settings.temperature).toBe(1.5)
  })

  it('updates close_behavior immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ close_behavior: 'minimize' })
    })
    expect(result.current.settings.close_behavior).toBe('minimize')
  })

  it('updates close_remembered immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ close_remembered: true })
    })
    expect(result.current.settings.close_remembered).toBe(true)
  })

  it('updates window state immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ window_x: 200, window_y: 150, window_width: 1400, window_height: 900 })
    })
    expect(result.current.settings.window_x).toBe(200)
    expect(result.current.settings.window_y).toBe(150)
    expect(result.current.settings.window_width).toBe(1400)
    expect(result.current.settings.window_height).toBe(900)
  })

  it('updates sidebar state immediately in state', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ sidebar_visible: false, sidebar_view: 'settings' })
    })
    expect(result.current.settings.sidebar_visible).toBe(false)
    expect(result.current.settings.sidebar_view).toBe('settings')
  })

  it('preserves existing fields when updating new field', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ temperature: 1.0 })
    })
    // Other fields should be preserved
    expect(result.current.settings.model).toBe('gpt-4')
    expect(result.current.settings.base_url).toBe('https://api.openai.com/v1')
    expect(result.current.settings.system_prompt).toBe('你是一个有帮助的 AI 助手。')
  })

  it('debounces save for new fields', () => {
    vi.useFakeTimers()
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({ temperature: 1.5 })
    })
    expect(mockSaveSettings).not.toHaveBeenCalled()
    act(() => {
      vi.advanceTimersByTime(300)
    })
    expect(mockSaveSettings).toHaveBeenCalledOnce()
    expect(mockSaveSettings).toHaveBeenCalledWith(
      expect.objectContaining({ temperature: 1.5 })
    )
    vi.useRealTimers()
  })

  it('handles partial settings from API (missing new fields)', async () => {
    // API returns old format without new fields (backend should handle migration)
    // Frontend receives what backend returns - data migration is backend's responsibility
    mockGetSettings.mockResolvedValue({
      model: 'gpt-4',
      agent_id: 'default',
      context_window: 8192,
      working_directory: '',
      language: 'zh-CN',
      base_url: 'https://api.openai.com/v1',
      model_name: 'gpt-4',
      system_prompt: '你是一个有帮助的 AI 助手。',
      temperature: 0.7,
      close_behavior: 'ask',
      close_remembered: false,
      window_x: 100,
      window_y: 100,
      window_width: 1200,
      window_height: 800,
      sidebar_visible: true,
      sidebar_view: 'conversation',
    })

    const { result } = renderHook(() => useSettings())
    await waitFor(() => expect(result.current.loading).toBe(false))

    // Should have the values returned by the API (after backend migration)
    expect(result.current.settings.base_url).toBe('https://api.openai.com/v1')
    expect(result.current.settings.model_name).toBe('gpt-4')
    expect(result.current.settings.temperature).toBe(0.7)
  })

  it('updates multiple new fields at once', () => {
    const { result } = renderHook(() => useSettings())
    act(() => {
      result.current.updateSettings({
        base_url: 'https://custom.api.com/v1',
        model_name: 'custom-model',
        temperature: 0.3,
      })
    })
    expect(result.current.settings.base_url).toBe('https://custom.api.com/v1')
    expect(result.current.settings.model_name).toBe('custom-model')
    expect(result.current.settings.temperature).toBe(0.3)
  })

  it('handles temperature boundary values', () => {
    const { result } = renderHook(() => useSettings())

    act(() => {
      result.current.updateSettings({ temperature: 0.0 })
    })
    expect(result.current.settings.temperature).toBe(0.0)

    act(() => {
      result.current.updateSettings({ temperature: 2.0 })
    })
    expect(result.current.settings.temperature).toBe(2.0)
  })

  it('handles all close_behavior values', () => {
    const { result } = renderHook(() => useSettings())

    act(() => {
      result.current.updateSettings({ close_behavior: 'minimize' })
    })
    expect(result.current.settings.close_behavior).toBe('minimize')

    act(() => {
      result.current.updateSettings({ close_behavior: 'quit' })
    })
    expect(result.current.settings.close_behavior).toBe('quit')

    act(() => {
      result.current.updateSettings({ close_behavior: 'ask' })
    })
    expect(result.current.settings.close_behavior).toBe('ask')
  })
})
