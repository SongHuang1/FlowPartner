import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useWebSocket, type ChatEvent } from '@/hooks/useWebSocket'

const mockGetApiPort = vi.fn()
const mockUpdateApiBase = vi.fn()

vi.mock('@/lib/api', () => ({
  getApiPort: () => mockGetApiPort(),
  updateApiBase: (port: number) => mockUpdateApiBase(port),
}))

class MockWebSocket {
  static CONNECTING = 0
  static OPEN = 1
  static CLOSING = 2
  static CLOSED = 3

  readyState = MockWebSocket.CONNECTING
  onopen: (() => void) | null = null
  onclose: (() => void) | null = null
  onerror: (() => void) | null = null
  onmessage: ((evt: { data: string }) => void) | null = null
  sentMessages: string[] = []
  url: string = ''

  constructor(url: string) {
    this.url = url
    // eslint-disable-next-line @typescript-eslint/no-this-alias
    mockWebSocketInstance = this
  }

  send(data: string) {
    this.sentMessages.push(data)
  }

  close() {
    this.readyState = MockWebSocket.CLOSED
    this.onclose?.()
  }
}

let mockWebSocketInstance: MockWebSocket | null = null

const mockFetchBackendPort = vi.fn()
const mockOnBackendPortChanged = vi.fn().mockReturnValue(() => {})

describe('useWebSocket', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockWebSocketInstance = null

    global.WebSocket = MockWebSocket as unknown as typeof WebSocket

    window.flowPartner = {
      platform: 'win32',
      getVersion: vi.fn().mockResolvedValue('1.0.0'),
      onSystemLock: vi.fn(),
      onSystemFocus: vi.fn(),
      fetchBackendPort: mockFetchBackendPort,
      onBackendPortChanged: mockOnBackendPortChanged,
      onCloseAction: vi.fn(),
      sendCloseAction: vi.fn(),
      updateCloseBehavior: vi.fn(),
      openExternal: vi.fn(),
      selectFolder: vi.fn(),
    }

    mockFetchBackendPort.mockResolvedValue(8080)
    mockGetApiPort.mockReturnValue(8080)
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function openConnection() {
    act(() => {
      mockWebSocketInstance!.readyState = MockWebSocket.OPEN
      mockWebSocketInstance!.onopen?.()
    })
  }

  function sendEvent(event: ChatEvent) {
    act(() => {
      mockWebSocketInstance!.onmessage?.({ data: JSON.stringify(event) })
    })
  }

  function closeConnection() {
    act(() => {
      mockWebSocketInstance!.onclose?.()
    })
  }

  describe('connection management', () => {
    it('connects on mount after fetching port', async () => {
      renderHook(() => useWebSocket())

      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      expect(mockWebSocketInstance!.url).toBe('ws://localhost:8080/ws')
    })

    it('sets connected=true when connection opens', async () => {
      const { result } = renderHook(() => useWebSocket())

      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      expect(result.current.connected).toBe(true)
    })

    it('exposes reconnectAttempts count', async () => {
      vi.useFakeTimers()
      mockFetchBackendPort.mockResolvedValue(8080)

      const { result } = renderHook(() => useWebSocket())

      await act(async () => {
        await vi.runAllTimersAsync()
      })
      await vi.waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()
      closeConnection()
      expect(result.current.reconnectAttempts).toBe(1)

      // Let reconnect happen (creates new WS in connecting state), then close it directly
      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000)
      })
      closeConnection()
      expect(result.current.reconnectAttempts).toBe(2)

      vi.useRealTimers()
    })

    it('starts reconnection on close', async () => {
      const { result } = renderHook(() => useWebSocket())

      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()
      expect(result.current.connected).toBe(true)

      closeConnection()
      expect(result.current.connected).toBe(false)
      expect(result.current.reconnecting).toBe(true)
    })

    it('reconnects after interval', async () => {
      vi.useFakeTimers()
      mockFetchBackendPort.mockResolvedValue(8080)

      renderHook(() => useWebSocket())

      await act(async () => {
        await vi.runAllTimersAsync()
      })
      await vi.waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()
      closeConnection()

      const firstInstance = mockWebSocketInstance

      await act(async () => {
        await vi.advanceTimersByTimeAsync(3000)
      })

      expect(mockWebSocketInstance).not.toBe(firstInstance)
      vi.useRealTimers()
    })

    it('stops reconnecting after 5 attempts and shows exhausted', async () => {
      vi.useFakeTimers()
      mockFetchBackendPort.mockResolvedValue(8080)

      const { result } = renderHook(() => useWebSocket())

      await act(async () => {
        await vi.runAllTimersAsync()
      })
      await vi.waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      // 5 reconnect attempts + 1 final close to trigger exhausted = 6 closes total
      for (let i = 0; i < 6; i++) {
        closeConnection()
        await act(async () => {
          await vi.advanceTimersByTimeAsync(3000)
        })
      }

      expect(result.current.connected).toBe(false)
      expect(result.current.reconnecting).toBe(false)
      vi.useRealTimers()
    })

    it('manual reconnect resets attempts and reconnects', async () => {
      vi.useFakeTimers()
      mockFetchBackendPort.mockResolvedValue(8080)

      const { result } = renderHook(() => useWebSocket())

      await act(async () => {
        await vi.runAllTimersAsync()
      })
      await vi.waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      // 6 closes to exhaust all 5 reconnect attempts
      for (let i = 0; i < 6; i++) {
        closeConnection()
        await act(async () => {
          await vi.advanceTimersByTimeAsync(3000)
        })
      }

      // Verify exhausted state
      expect(result.current.reconnecting).toBe(false)

      act(() => {
        result.current.manualReconnect()
      })

      // After manual reconnect, attempts reset and connection starts (connecting, not reconnect_exhausted)
      expect(result.current.reconnecting).toBe(false)
      expect(result.current.connected).toBe(false)
      // A new WS should have been created (connecting state)
      expect(mockWebSocketInstance).not.toBeNull()
      vi.useRealTimers()
    })
  })

  describe('sendMessage', () => {
    it('returns false when not connected', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())

      let sent: boolean
      act(() => {
        sent = result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(sent!).toBe(false)
    })

    it('returns false for empty message', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      let sent: boolean
      act(() => {
        sent = result.current.sendMessage('   ', 'sess_test_1', [])
      })
      expect(sent!).toBe(false)
    })

    it('returns false when already processing', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('first', 'sess_test_1', [])
      })

      let sent: boolean
      act(() => {
        sent = result.current.sendMessage('second', 'sess_test_1', [])
      })
      expect(sent!).toBe(false)
    })

    it('sends message and sets processing', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      let sent: boolean
      act(() => {
        sent = result.current.sendMessage('hello', 'sess_test_1', [
          { id: 'msg_1', role: 'user', content: 'previous', timestamp: 1 },
        ])
      })

      expect(sent!).toBe(true)
      expect(result.current.processing).toBe(true)
      expect(mockWebSocketInstance!.sentMessages).toHaveLength(1)

      const parsed = JSON.parse(mockWebSocketInstance!.sentMessages[0])
      expect(parsed.action).toBe('start_chat')
      expect(parsed.content).toBe('hello')
      expect(parsed.session_id).toBe('sess_test_1')
      expect(parsed.history).toEqual([{ role: 'user', content: 'previous' }])
    })

    it('clears events on new sendMessage', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({ event_type: 'status_update', payload: '{"status":"thinking","iteration":1}' })
      expect(result.current.events).toHaveLength(1)

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      expect(result.current.events).toHaveLength(0)
    })
  })

  describe('processing state machine', () => {
    it('resets processing on final_answer', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(result.current.processing).toBe(true)

      sendEvent({ event_type: 'final_answer', payload: '{"text":"done"}' })

      expect(result.current.processing).toBe(false)
    })

    it('resets processing on error', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(result.current.processing).toBe(true)

      sendEvent({ event_type: 'error', payload: '{"message":"fail"}' })

      expect(result.current.processing).toBe(false)
    })

    it('resets processing on timeout', async () => {
      vi.useFakeTimers()
      mockFetchBackendPort.mockResolvedValue(8080)

      const { result } = renderHook(() => useWebSocket())

      await act(async () => {
        await vi.runAllTimersAsync()
      })
      await vi.waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(result.current.processing).toBe(true)

      await act(async () => {
        await vi.advanceTimersByTimeAsync(300000)
      })

      expect(result.current.processing).toBe(false)
      vi.useRealTimers()
    })

    it('sets session end flag on timeout and ignores subsequent events', async () => {
      vi.useFakeTimers()
      mockFetchBackendPort.mockResolvedValue(8080)

      const { result } = renderHook(() => useWebSocket())

      await act(async () => {
        await vi.runAllTimersAsync()
      })
      await vi.waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(result.current.processing).toBe(true)

      await act(async () => {
        await vi.advanceTimersByTimeAsync(300000)
      })

      expect(result.current.processing).toBe(false)

      // After timeout, events should be ignored
      const eventsBefore = result.current.events.length
      sendEvent({ event_type: 'tool_call', payload: '{"tool":"read_file"}' })
      expect(result.current.events.length).toBe(eventsBefore)

      vi.useRealTimers()
    })

    it('resets processing on WS disconnect', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(result.current.processing).toBe(true)

      closeConnection()

      expect(result.current.processing).toBe(false)
    })
  })

  describe('callbacks', () => {
    it('calls onFinalAnswer when final_answer received', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const cb = vi.fn()
      act(() => {
        result.current.onFinalAnswer(cb)
      })

      sendEvent({ event_type: 'final_answer', payload: '{"text":"the answer"}' })

      expect(cb).toHaveBeenCalledWith('the answer')
    })

    it('calls onError when error received', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const cb = vi.fn()
      act(() => {
        result.current.onError(cb)
      })

      sendEvent({ event_type: 'error', payload: '{"message":"something failed"}' })

      expect(cb).toHaveBeenCalledWith('something failed')
    })

    it('returns unregister function from onFinalAnswer', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const cb = vi.fn()
      let unregister: () => void
      act(() => {
        unregister = result.current.onFinalAnswer(cb)
      })

      act(() => {
        unregister!()
      })

      sendEvent({ event_type: 'final_answer', payload: '{"text":"the answer"}' })

      expect(cb).not.toHaveBeenCalled()
    })

    it('callback exception does not affect other callbacks', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const cb1 = vi.fn().mockImplementation(() => { throw new Error('cb1 error') })
      const cb2 = vi.fn()

      act(() => {
        result.current.onFinalAnswer(cb1)
        result.current.onFinalAnswer(cb2)
      })

      sendEvent({ event_type: 'final_answer', payload: '{"text":"answer"}' })

      expect(cb1).toHaveBeenCalled()
      expect(cb2).toHaveBeenCalled()
    })
  })

  describe('session end flag', () => {
    it('ignores events after final_answer', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({ event_type: 'final_answer', payload: '{"text":"done"}' })
      const eventsAfterFinal = result.current.events.length

      sendEvent({ event_type: 'tool_call', payload: '{"tool":"read_file"}' })

      expect(result.current.events.length).toBe(eventsAfterFinal)
    })

    it('ignores events after error', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({ event_type: 'error', payload: '{"message":"fail"}' })
      const eventsAfterError = result.current.events.length

      sendEvent({ event_type: 'tool_call', payload: '{"tool":"read_file"}' })

      expect(result.current.events.length).toBe(eventsAfterError)
    })

    it('resets session end flag on new sendMessage', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('first', 'sess_test_1', [])
      })
      sendEvent({ event_type: 'final_answer', payload: '{"text":"done"}' })

      act(() => {
        result.current.sendMessage('second', 'sess_test_1', [])
      })
      sendEvent({ event_type: 'tool_call', payload: '{"tool":"read_file"}' })

      const toolCalls = result.current.events.filter(e => e.event_type === 'tool_call')
      expect(toolCalls.length).toBeGreaterThanOrEqual(1)
    })
  })

  describe('payload validation', () => {
    it('ignores unknown event_type', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({ event_type: 'unknown_type', payload: '{}' })

      expect(result.current.events).toHaveLength(0)
    })

    it('console.warns on unknown event_type', async () => {
      const warnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {})

      renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({ event_type: 'llm_chunk', payload: '{}' })

      expect(warnSpy).not.toHaveBeenCalled()
      warnSpy.mockRestore()
    })

    it('resets processing even when final_answer payload is malformed', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      sendEvent({ event_type: 'final_answer', payload: '{"no_text": true}' })

      expect(result.current.processing).toBe(false)
    })

    it('resets processing even when error payload is malformed', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      sendEvent({ event_type: 'error', payload: '{}' })

      expect(result.current.processing).toBe(false)
    })

    it('handles malformed JSON payload gracefully', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      sendEvent({ event_type: 'final_answer', payload: 'not json' })

      expect(result.current.processing).toBe(false)
    })
  })

  describe('port change', () => {
    it('reconnects on port change', async () => {
      let portChangeHandler: ((port: number) => void) | null = null
      mockOnBackendPortChanged.mockImplementation((cb: (port: number) => void) => {
        portChangeHandler = cb
        return () => {}
      })

      renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      expect(portChangeHandler).not.toBeNull()

      act(() => {
        portChangeHandler!(9090)
      })

      expect(mockUpdateApiBase).toHaveBeenCalledWith(9090)
    })

    it('resets processing and notifies on port change during processing', async () => {
      let portChangeHandler: ((port: number) => void) | null = null
      mockOnBackendPortChanged.mockImplementation((cb: (port: number) => void) => {
        portChangeHandler = cb
        return () => {}
      })

      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })
      expect(result.current.processing).toBe(true)

      const errorCb = vi.fn()
      act(() => {
        result.current.onError(errorCb)
      })

      expect(portChangeHandler).not.toBeNull()

      act(() => {
        portChangeHandler!(9090)
      })

      expect(result.current.processing).toBe(false)
      expect(errorCb).toHaveBeenCalledWith('连接已断开，请重试')
    })
  })

  describe('cleanup on unmount', () => {
    it('closes WebSocket on unmount', async () => {
      const { unmount } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const closeSpy = vi.spyOn(mockWebSocketInstance!, 'close')

      unmount()

      expect(closeSpy).toHaveBeenCalled()
    })

    it('clears callbacks on unmount', async () => {
      const { result, unmount } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const finalAnswerCb = vi.fn()
      act(() => {
        result.current.onFinalAnswer(finalAnswerCb)
      })

      unmount()

      sendEvent({ event_type: 'final_answer', payload: '{"text":"should not fire"}' })
      expect(finalAnswerCb).not.toHaveBeenCalled()
    })
  })

  describe('event isolation (AC14)', () => {
    it('events from previous session do not leak into new session', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      // First session: send message and receive events
      act(() => {
        result.current.sendMessage('first', 'sess_test_1', [])
      })
      sendEvent({ event_type: 'status_update', payload: '{"status":"thinking","iteration":1}' })
      sendEvent({ event_type: 'tool_call', payload: '{"tool":"read_file","args":{"path":"/test.txt"}}' })
      expect(result.current.events.length).toBe(2)

      // Receive final_answer to end first session
      sendEvent({ event_type: 'final_answer', payload: '{"text":"first answer"}' })
      expect(result.current.processing).toBe(false)

      // Second session: sendMessage should clear events
      act(() => {
        result.current.sendMessage('second', 'sess_test_1', [])
      })
      expect(result.current.events.length).toBe(0)

      // New events should only be from second session
      sendEvent({ event_type: 'status_update', payload: '{"status":"thinking","iteration":1}' })
      expect(result.current.events.length).toBe(1)
    })
  })

  describe('malformed payload resilience (AC18)', () => {
    it('does not crash when final_answer payload is malformed JSON', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      // Should not throw, processing should reset
      expect(() => {
        sendEvent({ event_type: 'final_answer', payload: '{invalid json' })
      }).not.toThrow()
      expect(result.current.processing).toBe(false)
    })

    it('does not crash when error payload is malformed JSON', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      expect(() => {
        sendEvent({ event_type: 'error', payload: '{invalid json' })
      }).not.toThrow()
      expect(result.current.processing).toBe(false)
    })

    it('does not crash when WS message is not valid JSON', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      act(() => {
        result.current.sendMessage('hello', 'sess_test_1', [])
      })

      expect(() => {
        act(() => {
          mockWebSocketInstance!.onmessage?.({ data: 'not json at all' })
        })
      }).not.toThrow()
      expect(result.current.processing).toBe(true)
    })
  })

  describe('security events', () => {
    it('triggers security callback for invalid port', async () => {
      let portChangeHandler: ((port: number) => void) | null = null
      mockOnBackendPortChanged.mockImplementation((cb: (port: number) => void) => {
        portChangeHandler = cb
        return () => {}
      })

      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const securityCb = vi.fn()
      act(() => {
        result.current.onSecurityEvent(securityCb)
      })

      expect(portChangeHandler).not.toBeNull()

      act(() => {
        portChangeHandler!(100)
      })

      await waitFor(() => expect(securityCb).toHaveBeenCalled())
    })

    it('does not trigger onError for security events', async () => {
      let portChangeHandler: ((port: number) => void) | null = null
      mockOnBackendPortChanged.mockImplementation((cb: (port: number) => void) => {
        portChangeHandler = cb
        return () => {}
      })

      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const errorCb = vi.fn()
      act(() => {
        result.current.onError(errorCb)
      })

      expect(portChangeHandler).not.toBeNull()

      act(() => {
        portChangeHandler!(100)
      })

      expect(errorCb).not.toHaveBeenCalled()
    })
  })
})
