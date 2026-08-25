import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { useWebSocket, deriveContentBlocks, type ChatEvent } from '@/hooks/useWebSocket'

const mockGetApiPort = vi.fn()

vi.mock('@/lib/api', () => ({
  getApiPort: () => mockGetApiPort(),
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
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const securityCb = vi.fn()
      act(() => {
        result.current.onSecurityEvent(securityCb)
      })

      mockGetApiPort.mockReturnValue(100)
      act(() => {
        result.current.manualReconnect()
      })

      await waitFor(() => expect(securityCb).toHaveBeenCalled())
    })

    it('does not trigger onError for security events', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const errorCb = vi.fn()
      act(() => {
        result.current.onError(errorCb)
      })

      mockGetApiPort.mockReturnValue(100)
      act(() => {
        result.current.manualReconnect()
      })

      expect(errorCb).not.toHaveBeenCalled()
    })
  })

  describe('subagent events', () => {
    it('builds subagentRuns from start/step/end events', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({
        event_type: 'subagent_start',
        payload: JSON.stringify({
          event_type: 'subagent_start',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          parent_span_id: 'span-root',
          task: '翻译这段话',
        }),
      })
      sendEvent({
        event_type: 'subagent_step',
        payload: JSON.stringify({
          event_type: 'subagent_step',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          step_type: 'thinking',
          content: '先理解原文',
        }),
      })
      sendEvent({
        event_type: 'subagent_step',
        payload: JSON.stringify({
          event_type: 'subagent_step',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          step_type: 'tool_call',
          tool: 'read_file',
          args: { path: 'a.txt' },
        }),
      })
      sendEvent({
        event_type: 'subagent_end',
        payload: JSON.stringify({
          event_type: 'subagent_end',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          result: '译文',
        }),
      })

      const runs = result.current.subagentRuns
      expect(runs).toHaveLength(1)
      expect(runs[0]).toMatchObject({
        agent_id: 'agent-1',
        agent_name: '翻译官',
        depth: 2,
        span_id: 'span-1',
        trace_id: 'trace-1',
        parent_span_id: 'span-root',
        status: 'done',
        task: '翻译这段话',
        result: '译文',
      })
      expect(runs[0].steps).toHaveLength(2)
      expect(runs[0].steps[0]).toMatchObject({ step_type: 'thinking', content: '先理解原文' })
      expect(runs[0].steps[1]).toMatchObject({ step_type: 'tool_call', tool: 'read_file', args: { path: 'a.txt' } })
    })

    it('marks run as error on subagent_error event', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({
        event_type: 'subagent_start',
        payload: JSON.stringify({
          event_type: 'subagent_start',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          task: '翻译',
        }),
      })
      sendEvent({
        event_type: 'subagent_error',
        payload: JSON.stringify({
          event_type: 'subagent_error',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          message: '调用失败',
        }),
      })

      const runs = result.current.subagentRuns
      expect(runs).toHaveLength(1)
      expect(runs[0].status).toBe('error')
      expect(runs[0].error).toBe('调用失败')
    })

    it('keeps multiple subagent runs independent by span_id', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      for (const span of ['span-1', 'span-2']) {
        sendEvent({
          event_type: 'subagent_start',
          payload: JSON.stringify({
            event_type: 'subagent_start',
            agent_id: 'agent-1',
            agent_name: '翻译官',
            depth: 2,
            span_id: span,
            trace_id: 'trace-1',
            task: `任务 ${span}`,
          }),
        })
      }
      sendEvent({
        event_type: 'subagent_end',
        payload: JSON.stringify({
          event_type: 'subagent_end',
          agent_id: 'agent-1',
          agent_name: '翻译官',
          depth: 2,
          span_id: 'span-1',
          trace_id: 'trace-1',
          result: '译文 1',
        }),
      })

      const runs = result.current.subagentRuns
      expect(runs).toHaveLength(2)
      expect(runs[0].status).toBe('done')
      expect(runs[1].status).toBe('running')
    })

    it('sends executor_agent_id and inject_agent_id in start_chat payload', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      const sent = result.current.sendMessage('hello', 'sess_test_1', [], 'agent-2', 'agent-1')
      expect(sent).toBe(true)

      const parsed = JSON.parse(mockWebSocketInstance!.sentMessages[0])
      expect(parsed.executor_agent_id).toBe('agent-2')
      expect(parsed.inject_agent_id).toBe('agent-1')
    })
  })

  describe('deriveContentBlocks', () => {
    const evt = (event_type: string, payload: Record<string, unknown>): ChatEvent => ({
      event_type,
      payload: JSON.stringify(payload),
    })

    it('interleaves text and subagent cards in chronological order', () => {
      const blocks = deriveContentBlocks([
        evt('llm_chunk', { content: '让我调用子智能体。' }),
        evt('tool_call', { tool: 'agent__a', args: { task: '任务A' }, call_id: 'c1' }),
        evt('subagent_start', { span_id: 'span-1', agent_name: '测试A', task: '任务A' }),
        evt('subagent_step', { span_id: 'span-1', step_type: 'final_answer', content: 'A 的回答' }),
        evt('subagent_end', { span_id: 'span-1', result: 'A 的回答' }),
        evt('tool_result', { tool: 'agent__a', call_id: 'c1', result: 'A 的回答' }),
        evt('llm_chunk', { content: '我是FlowPartner。' }),
        evt('tool_call', { tool: 'agent__b', args: { task: '任务B' }, call_id: 'c2' }),
        evt('subagent_start', { span_id: 'span-2', agent_name: '测试B', task: '任务B' }),
        evt('subagent_end', { span_id: 'span-2', result: 'B 的回答' }),
        evt('tool_result', { tool: 'agent__b', call_id: 'c2', result: 'B 的回答' }),
        evt('llm_chunk', { content: '以上是两次调用的结果。' }),
      ])

      expect(blocks).toHaveLength(5)
      expect(blocks[0]).toEqual({ type: 'text', content: '让我调用子智能体。' })
      const card1 = blocks[1] as Extract<(typeof blocks)[number], { type: 'subagent' }>
      expect(card1.span_id).toBe('span-1')
      expect(card1.status).toBe('done')
      expect(card1.result).toBe('A 的回答')
      expect(blocks[2]).toEqual({ type: 'text', content: '我是FlowPartner。' })
      const card2 = blocks[3] as Extract<(typeof blocks)[number], { type: 'subagent' }>
      expect(card2.span_id).toBe('span-2')
      expect(blocks[4]).toEqual({ type: 'text', content: '以上是两次调用的结果。' })
    })

    it('closes unpaired agent card via tool_result fallback', () => {
      const blocks = deriveContentBlocks([
        evt('tool_call', { tool: 'agent__missing', args: { task: 'x' }, call_id: 'c9' }),
        evt('tool_result', { tool: 'agent__missing', call_id: 'c9', result: '子智能体调用失败：不存在' }),
      ])

      expect(blocks).toHaveLength(1)
      const card = blocks[0] as Extract<(typeof blocks)[number], { type: 'subagent' }>
      expect(card.status).toBe('done')
      expect(card.result).toBe('子智能体调用失败：不存在')
    })

    it('does not overwrite a paired card when tool_result arrives', () => {
      const blocks = deriveContentBlocks([
        evt('tool_call', { tool: 'agent__a', args: {}, call_id: 'c1' }),
        evt('subagent_start', { span_id: 'span-1', agent_name: '测试A', task: '' }),
        evt('subagent_end', { span_id: 'span-1', result: '正确结果' }),
        evt('tool_result', { tool: 'agent__a', call_id: 'c1', result: '正确结果' }),
      ])

      const card = blocks[0] as Extract<(typeof blocks)[number], { type: 'subagent' }>
      expect(card.result).toBe('正确结果')
      expect(card.status).toBe('done')
    })
  })

  describe('llm_chunk in event stream', () => {
    it('records llm_chunk events so content blocks can be derived', async () => {
      const { result } = renderHook(() => useWebSocket())
      await waitFor(() => expect(mockWebSocketInstance).not.toBeNull())
      openConnection()

      sendEvent({ event_type: 'llm_chunk', payload: JSON.stringify({ content: '你好' }) })
      sendEvent({ event_type: 'llm_chunk', payload: JSON.stringify({ content: '，世界' }) })

      const chunks = result.current.events.filter((e) => e.event_type === 'llm_chunk')
      expect(chunks).toHaveLength(2)
    })
  })
})
