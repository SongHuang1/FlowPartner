import { describe, it, expect, vi, beforeEach } from 'vitest'
import { unlock, lock, getLockStatus, initApi } from '@/lib/api'
import type { LockStatus } from '@/types'

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

describe('api - unlock/lock/lockStatus', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    initApi(8080)
  })

  describe('unlock', () => {
    it('sends POST request with password', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: { message: '解锁成功' }, timestamp: 123, request_id: 'uuid' })
      )

      await unlock('TestPass123')

      expect(mockFetch).toHaveBeenCalledWith('http://localhost:8080/api/unlock', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password: 'TestPass123' }),
        signal: expect.any(AbortSignal),
      })
    })

    it('throws error on wrong password', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 5003, message: '密码错误', data: null, timestamp: 123, request_id: 'uuid' })
      )

      await expect(unlock('WrongPass123')).rejects.toThrow('密码错误')
    })

    it('throws error on rate limit', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 5001, message: 'Too many failed attempts', data: null, timestamp: 123, request_id: 'uuid' })
      )

      await expect(unlock('AnyPass123')).rejects.toThrow('Too many failed attempts')
    })

    it('throws error when API key not configured', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 5002, message: '请先配置 API Key', data: null, timestamp: 123, request_id: 'uuid' })
      )

      await expect(unlock('AnyPass123')).rejects.toThrow('请先配置 API Key')
    })

    it('throws error on network failure', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'))

      await expect(unlock('TestPass123')).rejects.toThrow('Network error')
    })

    it('throws error on server error', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 2001, message: 'Internal error', data: null, timestamp: 123, request_id: 'uuid' }, false, 500)
      )

      await expect(unlock('TestPass123')).rejects.toThrow('Internal error')
    })
  })

  describe('lock', () => {
    it('sends POST request', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: null, timestamp: 123, request_id: 'uuid' })
      )

      await lock()

      expect(mockFetch).toHaveBeenCalledWith('http://localhost:8080/api/lock', {
        method: 'POST',
        signal: expect.any(AbortSignal),
      })
    })

    it('throws error on failure', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 2001, message: 'Lock failed', data: null, timestamp: 123, request_id: 'uuid' }, false, 500)
      )

      await expect(lock()).rejects.toThrow('Lock failed')
    })

    it('throws error on network failure', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'))

      await expect(lock()).rejects.toThrow('Network error')
    })
  })

  describe('getLockStatus', () => {
    it('returns lock status on success', async () => {
      const mockStatus: LockStatus = {
        locked: true,
        failed_attempts: 0,
        has_api_key: false,
      }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: mockStatus, timestamp: 123, request_id: 'uuid' })
      )

      const result = await getLockStatus()

      expect(result).toEqual(mockStatus)
      expect(mockFetch).toHaveBeenCalledWith('http://localhost:8080/api/lock_status', expect.any(Object))
    })

    it('returns locked status with locked_until', async () => {
      const mockStatus: LockStatus = {
        locked: true,
        locked_until: '2026-07-23T12:00:00Z',
        failed_attempts: 5,
        has_api_key: true,
      }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: mockStatus, timestamp: 123, request_id: 'uuid' })
      )

      const result = await getLockStatus()

      expect(result.locked).toBe(true)
      expect(result.locked_until).toBe('2026-07-23T12:00:00Z')
      expect(result.failed_attempts).toBe(5)
      expect(result.has_api_key).toBe(true)
    })

    it('returns unlocked status', async () => {
      const mockStatus: LockStatus = {
        locked: false,
        failed_attempts: 0,
        has_api_key: true,
      }
      mockFetch.mockResolvedValue(
        mockResponse({ code: 0, message: 'success', data: mockStatus, timestamp: 123, request_id: 'uuid' })
      )

      const result = await getLockStatus()

      expect(result.locked).toBe(false)
      expect(result.has_api_key).toBe(true)
    })

    it('throws error on network failure', async () => {
      mockFetch.mockRejectedValue(new Error('Network error'))

      await expect(getLockStatus()).rejects.toThrow('Network error')
    })

    it('throws error on server error', async () => {
      mockFetch.mockResolvedValue(
        mockResponse({ code: 2001, message: 'Server error', data: null, timestamp: 123, request_id: 'uuid' }, false, 500)
      )

      await expect(getLockStatus()).rejects.toThrow('Server error')
    })
  })

})
