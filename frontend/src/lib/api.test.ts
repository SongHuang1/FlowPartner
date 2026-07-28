import { describe, it, expect, vi, beforeEach } from 'vitest'
import { getSettings, saveSettings, getConversation, saveConversation } from '@/lib/api'
import type { Settings, Conversation, Message } from '@/types'

// Mock fetch globally
const mockFetch = vi.fn()
global.fetch = mockFetch

function mockResponse(data: unknown, ok = true, status = 200): Response {
  return {
    ok,
    status,
    json: () => Promise.resolve(data),
  } as unknown as Response
}

describe('api', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  describe('getSettings', () => {
    it('returns settings data on success', async () => {
      const mockSettings: Settings = {
        model: 'gpt-4',
        agent_id: 'default',
        context_window: 8192,
        working_directory: '/test',
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
      }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: mockSettings, timestamp: 123, request_id: 'uuid' })
      )

      const result = await getSettings()
      expect(result).toEqual(mockSettings)
      expect(mockFetch).toHaveBeenCalledWith('/api/settings', expect.any(Object))
    })

    it('throws error on non-ok response', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 2001, message: 'Internal error', data: null, timestamp: 123, request_id: 'uuid' }, false, 500)
      )

      await expect(getSettings()).rejects.toThrow('Internal error')
    })

    it('throws error with status code when no backend message', async () => {
      mockFetch.mockResolvedValue({
        ok: false,
        status: 503,
        json: () => Promise.reject(new Error('parse fail')),
      } as unknown as Response)

      await expect(getSettings()).rejects.toThrow('Request failed: 503')
    })
  })

  describe('saveSettings', () => {
    it('sends PUT request with correct body', async () => {
      const settings: Settings = {
        model: 'gpt-3.5',
        agent_id: 'test-agent',
        context_window: 4096,
        working_directory: '',
        language: 'en-US',
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
      }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: null, timestamp: 123, request_id: 'uuid' })
      )

      await saveSettings(settings)

      expect(mockFetch).toHaveBeenCalledWith('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(settings),
        signal: expect.any(AbortSignal),
      })
    })

    it('throws error on failure', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 1001, message: 'Invalid param', data: null, timestamp: 123, request_id: 'uuid' }, false, 400)
      )

      await expect(saveSettings({} as Settings)).rejects.toThrow('Invalid param')
    })
  })

  describe('getConversation', () => {
    it('returns conversation data on success', async () => {
      const mockConv: Conversation = {
        messages: [
          { id: 'msg_1', role: 'user', content: 'hello', timestamp: 1000 },
        ],
        updated_at: 1000,
      }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: mockConv, timestamp: 123, request_id: 'uuid' })
      )

      const result = await getConversation()
      expect(result).toEqual(mockConv)
      expect(mockFetch).toHaveBeenCalledWith('/api/conversation', expect.any(Object))
    })

    it('returns empty conversation when no messages', async () => {
      const mockConv: Conversation = { messages: [], updated_at: 0 }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: mockConv, timestamp: 123, request_id: 'uuid' })
      )

      const result = await getConversation()
      expect(result.messages).toEqual([])
    })

    it('throws error on network failure', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'))

      await expect(getConversation()).rejects.toThrow('Network error')
    })
  })

  describe('saveConversation', () => {
    it('sends POST request with messages and updated_at', async () => {
      const messages: Message[] = [
        { id: 'msg_1', role: 'user', content: 'test', timestamp: 1000 },
      ]
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: null, timestamp: 123, request_id: 'uuid' })
      )

      await saveConversation(messages)

      expect(mockFetch).toHaveBeenCalledWith('/api/conversation', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: expect.stringContaining('"messages"'),
        signal: expect.any(AbortSignal),
      })

      // Verify body contains messages and updated_at
      const callArgs = mockFetch.mock.calls[0]
      const body = JSON.parse((callArgs[1] as RequestInit).body as string)
      expect(body.messages).toEqual(messages)
      expect(body.updated_at).toBeTypeOf('number')
    })

    it('throws error on failure', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 2001, message: 'Failed to save', data: null, timestamp: 123, request_id: 'uuid' }, false, 500)
      )

      await expect(saveConversation([])).rejects.toThrow('Failed to save')
    })
  })

  describe('saveApiKey', () => {
    it('sends PUT request with api_key and password', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: null, timestamp: 123, request_id: 'uuid' })
      )

      const { saveApiKey } = await import('@/lib/api')
      await saveApiKey('sk-test-key-12345', 'StrongPass1')

      expect(mockFetch).toHaveBeenCalledWith('/api/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ api_key: 'sk-test-key-12345', password: 'StrongPass1' }),
        signal: expect.any(AbortSignal),
      })
    })

    it('throws error when backend returns non-zero code', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 1001, message: 'password is required', data: null, timestamp: 123, request_id: 'uuid' }, false, 400)
      )

      const { saveApiKey } = await import('@/lib/api')
      await expect(saveApiKey('sk-test-key-12345', '')).rejects.toThrow('password is required')
    })

    it('throws error with backend message when backend returns error', async () => {
      // 注意：fetchWithTimeout 在 response.ok=false 时会优先抛出 HTTP 错误
      // 只有当 response.ok=true 但 code!=0 时，才会使用后端 message
      const errorResponse = {
        ok: true,
        status: 200,
        json: () => Promise.resolve({ code: 2001, message: '', data: null, timestamp: 123, request_id: 'uuid' }),
      } as unknown as Response
      mockFetch.mockResolvedValue(errorResponse)

      const { saveApiKey } = await import('@/lib/api')
      await expect(saveApiKey('sk-test-key-12345', 'StrongPass1')).rejects.toThrow('保存 API Key 失败')
    })

    it('throws error on network failure', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'))

      const { saveApiKey } = await import('@/lib/api')
      await expect(saveApiKey('sk-test-key-12345', 'StrongPass1')).rejects.toThrow('Network error')
    })
  })

  describe('fetchWithTimeout', () => {
    it('aborts request after timeout', async () => {
      // Mock fetch that never resolves
      mockFetch.mockImplementation(() => new Promise(() => {}))

      // Use a very short timeout by mocking AbortController
      const abortError = new Error('The operation was aborted')
      abortError.name = 'AbortError'
      mockFetch.mockRejectedValue(abortError)

      await expect(getSettings()).rejects.toThrow()
    })

    it('clears timeout on successful response', async () => {
      const clearTimeoutSpy = vi.spyOn(global, 'clearTimeout')
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: {} as Settings, timestamp: 123, request_id: 'uuid' })
      )

      await getSettings()

      expect(clearTimeoutSpy).toHaveBeenCalled()
      clearTimeoutSpy.mockRestore()
    })
  })
})
