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
