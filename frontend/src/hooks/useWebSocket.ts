import { useState, useEffect, useRef, useCallback } from 'react'
import { getApiPort } from '@/lib/api'
import type { Message, PermissionRequestPayload, IterationStep, SnapshotStatus, SnapshotMessage, SubAgentRun, SubAgentStep } from '@/types'

export interface ChatEvent {
  event_type: string
  payload: string
}

type ConnectionState =
  | 'disconnected'
  | 'connecting'
  | 'connected'
  | 'reconnecting'
  | 'reconnect_exhausted'

const MAX_RECONNECT_ATTEMPTS = 5
const RECONNECT_INTERVAL_MS = 3000
const PROCESSING_TIMEOUT_MS = 300000
const MIN_PORT = 1024
const MAX_PORT = 65535

const KNOWN_EVENT_TYPES = [
  'status_update', 'tool_call', 'tool_result', 'final_answer',
  'error', 'permission_request', 'iteration_start', 'llm_chunk',
  'loop_terminated', 'snapshot_status', 'snapshot_message',
  'subagent_start', 'subagent_step', 'subagent_end', 'subagent_error',
  'agents_changed',
] as const

function isKnownEventType(type: string): type is (typeof KNOWN_EVENT_TYPES)[number] {
  return (KNOWN_EVENT_TYPES as readonly string[]).includes(type)
}

export interface UseWebSocketReturn {
  connected: boolean
  reconnecting: boolean
  reconnectAttempts: number
  isReconnectExhausted: boolean
  processing: boolean
  sendMessage: (content: string, sessionId: string, history: Message[], executorAgentId?: string, injectAgentId?: string) => boolean
  sendCancel: (sessionId: string) => void
  sendPermissionResponse: (sessionId: string, requestId: string, decision: 'allow' | 'deny', scope?: 'once' | 'session') => void
  sendManualSnapshot: () => void
  sendRestore: (snapshotId: string, deleteExtras: boolean) => void
  sendSystemLock: () => void
  events: ChatEvent[]
  steps: IterationStep[]
  subagentRuns: SubAgentRun[]
  manualReconnect: () => void
  onStreamChunk: (cb: (chunk: string) => void) => () => void
  onFinalAnswer: (cb: (answer: string) => void) => () => void
  onSubAgentStart: (cb: (info: { span_id: string; agent_name: string; task: string }) => void) => () => void
  onSubAgentStreamChunk: (cb: (span_id: string, chunk: string) => void) => () => void
  onSubAgentEnd: (cb: (span_id: string, result: string) => void) => () => void
  onAgentsChanged: (cb: () => void) => () => void
  onError: (cb: (message: string) => void) => () => void
  onSecurityEvent: (cb: (message: string) => void) => () => void
  onPermissionRequest: (cb: (payload: PermissionRequestPayload) => void) => () => void
  onSnapshotStatus: (cb: (status: SnapshotStatus) => void) => () => void
  onSnapshotMessage: (cb: (message: SnapshotMessage) => void) => () => void
}

function buildSteps(events: ChatEvent[]): IterationStep[] {
  const steps: IterationStep[] = []
  let current: IterationStep | null = null

  for (const evt of events) {
    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(evt.payload)
    } catch {
      continue
    }

    if (evt.event_type === 'iteration_start') {
      current = {
        iteration: (parsed.iteration as number) || steps.length + 1,
        thinking: '',
        toolCalls: [],
      }
      steps.push(current)
      continue
    }

    if (evt.event_type === 'llm_chunk') {
      if (!current) {
        current = { iteration: steps.length + 1, thinking: '', toolCalls: [] }
        steps.push(current)
      }
      current.thinking += (parsed.content as string) || ''
      continue
    }

    if (evt.event_type === 'tool_call') {
      if (!current) {
        current = { iteration: steps.length + 1, thinking: '', toolCalls: [] }
        steps.push(current)
      }
      current.toolCalls.push({
        tool: (parsed.tool as string) || '',
        args: (parsed.args as Record<string, unknown>) || {},
        call_id: (parsed.call_id as string) || '',
      })
      continue
    }

    if (evt.event_type === 'tool_result') {
      const callId = (parsed.call_id as string) || ''
      for (const step of steps) {
        const tc = step.toolCalls.find((t) => t.call_id === callId)
        if (tc) {
          tc.result = (parsed.result as string) || ''
          tc.truncated = parsed.truncated as boolean | undefined
          break
        }
      }
      continue
    }

    if (evt.event_type === 'loop_terminated') {
      if (!current) {
        current = { iteration: steps.length + 1, thinking: '', toolCalls: [] }
        steps.push(current)
      }
      current.loopTerminated = { reason: (parsed.reason as string) || '' }
    }
  }

  return steps
}

function buildSubagentRuns(events: ChatEvent[]): SubAgentRun[] {
  const runs = new Map<string, SubAgentRun>()
  const order: string[] = []

  for (const evt of events) {
    let parsed: Record<string, unknown>
    try {
      parsed = JSON.parse(evt.payload)
    } catch {
      continue
    }

    const spanId = (parsed.span_id as string) || ''
    if (!spanId) continue

    let run = runs.get(spanId)
    if (evt.event_type === 'subagent_start') {
      if (!run) {
        run = {
          agent_id: (parsed.agent_id as string) || '',
          agent_name: (parsed.agent_name as string) || '',
          depth: (parsed.depth as number) || 1,
          span_id: spanId,
          trace_id: (parsed.trace_id as string) || '',
          parent_span_id: (parsed.parent_span_id as string) || undefined,
          status: 'running',
          task: (parsed.task as string) || '',
          steps: [],
        }
        runs.set(spanId, run)
        order.push(spanId)
      }
      continue
    }

    if (!run) continue

    if (evt.event_type === 'subagent_step') {
      const step: SubAgentStep = {
        step_type: (parsed.step_type as SubAgentStep['step_type']) || 'thinking',
      }
      if (typeof parsed.content === 'string') step.content = parsed.content
      if (typeof parsed.tool === 'string') step.tool = parsed.tool
      if (parsed.args && typeof parsed.args === 'object') step.args = parsed.args as Record<string, unknown>
      if (typeof parsed.result === 'string') step.result = parsed.result
      if (typeof parsed.truncated === 'boolean') step.truncated = parsed.truncated
      run.steps.push(step)
      continue
    }

    if (evt.event_type === 'subagent_end') {
      run.status = 'done'
      if (typeof parsed.result === 'string') run.result = parsed.result
      continue
    }

    if (evt.event_type === 'subagent_error') {
      run.status = 'error'
      run.error = (parsed.message as string) || '未知错误'
    }
  }

  return order.map((id) => runs.get(id)!).filter(Boolean)
}

export function useWebSocket(): UseWebSocketReturn {
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected')
  const [events, setEvents] = useState<ChatEvent[]>([])
  const [processing, setProcessing] = useState(false)
  const [reconnectAttempts, setReconnectAttempts] = useState(0)

  const wsRef = useRef<WebSocket | null>(null)
  const reconnectAttemptsRef = useRef(0)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const processingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const sessionEndedRef = useRef(false)
  const mountedRef = useRef(true)
  const finalAnswerCallbacksRef = useRef<Set<(answer: string) => void>>(new Set())
  const streamChunkCallbacksRef = useRef<Set<(chunk: string) => void>>(new Set())
  const subAgentStartCallbacksRef = useRef<Set<(info: { span_id: string; agent_name: string; task: string }) => void>>(new Set())
  const subAgentStreamChunkCallbacksRef = useRef<Set<(span_id: string, chunk: string) => void>>(new Set())
  const subAgentEndCallbacksRef = useRef<Set<(span_id: string, result: string) => void>>(new Set())
  const agentsChangedCallbacksRef = useRef<Set<() => void>>(new Set())
  const errorCallbacksRef = useRef<Set<(message: string) => void>>(new Set())
  const securityCallbacksRef = useRef<Set<(message: string) => void>>(new Set())
  const permissionRequestCallbacksRef = useRef<Set<(payload: PermissionRequestPayload) => void>>(new Set())
  const snapshotStatusCallbacksRef = useRef<Set<(status: SnapshotStatus) => void>>(new Set())
  const snapshotMessageCallbacksRef = useRef<Set<(message: SnapshotMessage) => void>>(new Set())
  const connectRef = useRef<(port: number) => void>(() => {})
  const sessionIdRef = useRef<string>('')

  const connected = connectionState === 'connected'
  const reconnecting = connectionState === 'reconnecting'
  const isReconnectExhausted = connectionState === 'reconnect_exhausted'
  const steps = buildSteps(events)
  const subagentRuns = buildSubagentRuns(events)

  const clearProcessingTimer = useCallback(() => {
    if (processingTimerRef.current) {
      clearTimeout(processingTimerRef.current)
      processingTimerRef.current = null
    }
  }, [])

  const clearReconnectTimer = useCallback(() => {
    if (reconnectTimerRef.current) {
      clearTimeout(reconnectTimerRef.current)
      reconnectTimerRef.current = null
    }
  }, [])

  const resetProcessing = useCallback(() => {
    clearProcessingTimer()
    setProcessing(false)
  }, [clearProcessingTimer])

  const connect = useCallback(
    (port: number) => {
      if (!mountedRef.current) return

      if (port < MIN_PORT || port > MAX_PORT) {
        const msg = `端口 ${port} 不在安全范围（${MIN_PORT}-${MAX_PORT}），已拒绝连接`
        securityCallbacksRef.current.forEach((cb) => {
          try { cb(msg) } catch (e) { console.error('Security callback error:', e) }
        })
        return
      }

      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }

      setConnectionState(
        reconnectAttemptsRef.current > 0 ? 'reconnecting' : 'connecting',
      )

      const ws = new WebSocket(`ws://localhost:${port}/ws`)

      ws.onopen = () => {
        if (!mountedRef.current) { ws.close(); return }
        reconnectAttemptsRef.current = 0
        setReconnectAttempts(0)
        setConnectionState('connected')
      }

      ws.onmessage = (evt) => {
        if (!mountedRef.current) return

        let raw: ChatEvent
        try {
          raw = JSON.parse(evt.data)
        } catch {
          console.error('Failed to parse WebSocket message:', evt.data)
          return
        }

        console.log('[WS] Received event:', raw.event_type, raw.payload?.slice(0, 200))

        if (!raw.event_type || !isKnownEventType(raw.event_type)) {
          console.warn('Unknown event_type:', raw.event_type)
          return
        }

        // 快照事件是全局事件（与对话无关），不进入会话事件流，也不受会话结束状态门控
        if (raw.event_type === 'snapshot_status') {
          try {
            const parsed = JSON.parse(raw.payload) as SnapshotStatus
            snapshotStatusCallbacksRef.current.forEach((cb) => {
              try { cb(parsed) } catch (e) { console.error('onSnapshotStatus callback error:', e) }
            })
          } catch {
            console.error('Failed to parse snapshot_status payload:', raw.payload)
          }
          return
        }

        if (raw.event_type === 'snapshot_message') {
          try {
            const parsed = JSON.parse(raw.payload) as SnapshotMessage
            snapshotMessageCallbacksRef.current.forEach((cb) => {
              try { cb(parsed) } catch (e) { console.error('onSnapshotMessage callback error:', e) }
            })
          } catch {
            console.error('Failed to parse snapshot_message payload:', raw.payload)
          }
          return
        }

        if (sessionEndedRef.current) return

        if (raw.event_type === 'llm_chunk') {
          try {
            const parsed = JSON.parse(raw.payload)
            const content = (parsed.content as string) || ''
            streamChunkCallbacksRef.current.forEach((cb) => {
              try { cb(content) } catch (e) { console.error('onStreamChunk callback error:', e) }
            })
          } catch {
            console.error('Failed to parse llm_chunk payload:', raw.payload)
          }
          return
        }

        if (raw.event_type === 'final_answer' || raw.event_type === 'error') {
          resetProcessing()
          sessionEndedRef.current = true
        }

        if (raw.event_type === 'permission_request') {
          setEvents((prev) => [...prev, raw])
          try {
            const parsed = JSON.parse(raw.payload) as PermissionRequestPayload
            permissionRequestCallbacksRef.current.forEach((cb) => {
              try { cb(parsed) } catch (e) { console.error('onPermissionRequest callback error:', e) }
            })
          } catch {
            console.error('Failed to parse permission_request payload:', raw.payload)
          }
          return
        }

        if (raw.event_type === 'final_answer') {
          setEvents((prev) => [...prev, raw])
          let answer: string
          try {
            const parsed = JSON.parse(raw.payload)
            if (typeof parsed.text !== 'string') {
              console.error('final_answer payload missing text field')
              return
            }
            answer = parsed.text
          } catch {
            console.error('Failed to parse final_answer payload:', raw.payload)
            return
          }
          finalAnswerCallbacksRef.current.forEach((cb) => {
            try { cb(answer) } catch (e) { console.error('onFinalAnswer callback error:', e) }
          })
          return
        }

        // 子智能体事件处理
        if (raw.event_type === 'subagent_start') {
          setEvents((prev) => [...prev, raw])
          try {
            const parsed = JSON.parse(raw.payload)
            subAgentStartCallbacksRef.current.forEach((cb) => {
              try { cb({ span_id: parsed.span_id || '', agent_name: parsed.agent_name || '', task: parsed.task || '' }) } catch (e) { console.error('onSubAgentStart callback error:', e) }
            })
          } catch {
            console.error('Failed to parse subagent_start payload:', raw.payload)
          }
          return
        }

        if (raw.event_type === 'subagent_step') {
          setEvents((prev) => [...prev, raw])
          try {
            const parsed = JSON.parse(raw.payload)
            if (parsed.step_type === 'final_answer' && parsed.content) {
              const spanId = parsed.span_id || ''
              subAgentStreamChunkCallbacksRef.current.forEach((cb) => {
                try { cb(spanId, parsed.content) } catch (e) { console.error('onSubAgentStreamChunk callback error:', e) }
              })
            }
          } catch {
            console.error('Failed to parse subagent_step payload:', raw.payload)
          }
          return
        }

        if (raw.event_type === 'subagent_end') {
          setEvents((prev) => [...prev, raw])
          try {
            const parsed = JSON.parse(raw.payload)
            const spanId = parsed.span_id || ''
            subAgentEndCallbacksRef.current.forEach((cb) => {
              try { cb(spanId, parsed.result || '') } catch (e) { console.error('onSubAgentEnd callback error:', e) }
            })
          } catch {
            console.error('Failed to parse subagent_end payload:', raw.payload)
          }
          return
        }

        if (raw.event_type === 'subagent_error') {
          setEvents((prev) => [...prev, raw])
          return
        }

        if (raw.event_type === 'agents_changed') {
          agentsChangedCallbacksRef.current.forEach((cb) => {
            try { cb() } catch (e) { console.error('onAgentsChanged callback error:', e) }
          })
          return
        }

        if (raw.event_type === 'error') {
          setEvents((prev) => [...prev, raw])
          let message: string
          try {
            const parsed = JSON.parse(raw.payload)
            if (typeof parsed.message !== 'string') {
              console.error('error payload missing message field')
              return
            }
            message = parsed.message
          } catch {
            console.error('Failed to parse error payload:', raw.payload)
            return
          }
          errorCallbacksRef.current.forEach((cb) => {
            try { cb(message) } catch (e) { console.error('onError callback error:', e) }
          })
          return
        }

        setEvents((prev) => [...prev, raw])
      }

      ws.onclose = () => {
        if (!mountedRef.current) return
        wsRef.current = null
        resetProcessing()

        if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
          setConnectionState('reconnect_exhausted')
          return
        }

        reconnectAttemptsRef.current += 1
        setReconnectAttempts(reconnectAttemptsRef.current)
        setConnectionState('reconnecting')
        clearReconnectTimer()
        reconnectTimerRef.current = setTimeout(() => {
          const port = getApiPort()
          if (port) connectRef.current(port)
        }, RECONNECT_INTERVAL_MS)
      }

      ws.onerror = () => { ws.close() }

      wsRef.current = ws
    },
    [clearReconnectTimer, resetProcessing],
  )

  useEffect(() => {
    connectRef.current = connect
  })

  useEffect(() => {
    mountedRef.current = true

    const port = getApiPort()
    if (port) {
      connectRef.current(port)
    }

    const finalAnswerCbs = finalAnswerCallbacksRef.current
    const streamChunkCbs = streamChunkCallbacksRef.current
    const subAgentStartCbs = subAgentStartCallbacksRef.current
    const subAgentStreamChunkCbs = subAgentStreamChunkCallbacksRef.current
    const subAgentEndCbs = subAgentEndCallbacksRef.current
    const agentsChangedCbs = agentsChangedCallbacksRef.current
    const errorCbs = errorCallbacksRef.current
    const securityCbs = securityCallbacksRef.current
    const permissionCbs = permissionRequestCallbacksRef.current
    const snapshotStatusCbs = snapshotStatusCallbacksRef.current
    const snapshotMessageCbs = snapshotMessageCallbacksRef.current

    return () => {
      mountedRef.current = false
      clearReconnectTimer()
      clearProcessingTimer()
      if (wsRef.current) {
        wsRef.current.close()
        wsRef.current = null
      }
      finalAnswerCbs.clear()
      streamChunkCbs.clear()
      subAgentStartCbs.clear()
      subAgentStreamChunkCbs.clear()
      subAgentEndCbs.clear()
      agentsChangedCbs.clear()
      errorCbs.clear()
      securityCbs.clear()
      permissionCbs.clear()
      snapshotStatusCbs.clear()
      snapshotMessageCbs.clear()
      setEvents([])
    }
  }, [clearReconnectTimer, clearProcessingTimer, resetProcessing])

  const sendMessage = useCallback(
    (content: string, sessionId: string, history: Message[], executorAgentId?: string, injectAgentId?: string): boolean => {
      const trimmed = content.trim()
      if (!trimmed) return false
      if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return false
      if (processing) return false

      sessionIdRef.current = sessionId
      setEvents([])
      sessionEndedRef.current = false
      setProcessing(true)
      clearProcessingTimer()
      processingTimerRef.current = setTimeout(() => {
        resetProcessing()
        sessionEndedRef.current = true
        errorCallbacksRef.current.forEach((cb) => {
          try { cb('请求超时，请重试') } catch (e) { console.error('onError callback error:', e) }
        })
      }, PROCESSING_TIMEOUT_MS)

      const historyPayload = history.map((m) => {
        const entry: Record<string, unknown> = { role: m.role, content: m.content }
        if (m.tool_calls && m.tool_calls.length > 0) {
          entry.tool_calls = m.tool_calls
        }
        if (m.tool_call_id) {
          entry.tool_call_id = m.tool_call_id
        }
        if (m.name) {
          entry.name = m.name
        }
        return entry
      })

      const msg = JSON.stringify({
        action: 'start_chat',
        content: trimmed,
        session_id: sessionId,
        history: historyPayload,
        executor_agent_id: executorAgentId || '',
        inject_agent_id: injectAgentId || '',
      })
      wsRef.current.send(msg)
      return true
    },
    [processing, clearProcessingTimer, resetProcessing],
  )

  const sendCancel = useCallback((sessionId: string) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    wsRef.current.send(JSON.stringify({
      action: 'cancel_task',
      session_id: sessionId,
    }))
  }, [])

  const sendPermissionResponse = useCallback((sessionId: string, requestId: string, decision: 'allow' | 'deny', scope?: 'once' | 'session') => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    const payload: Record<string, string> = {
      action: 'permission_response',
      session_id: sessionId,
      request_id: requestId,
      decision,
    }
    if (decision === 'allow' && scope) {
      payload.scope = scope
    }
    wsRef.current.send(JSON.stringify(payload))
  }, [])

  const sendManualSnapshot = useCallback(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    wsRef.current.send(JSON.stringify({ action: 'manual_snapshot' }))
  }, [])

  const sendRestore = useCallback((snapshotId: string, deleteExtras: boolean) => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    wsRef.current.send(JSON.stringify({
      action: 'restore',
      snapshot_id: snapshotId,
      delete_extras: deleteExtras,
    }))
  }, [])

  const sendSystemLock = useCallback(() => {
    if (!wsRef.current || wsRef.current.readyState !== WebSocket.OPEN) return
    wsRef.current.send(JSON.stringify({ action: 'system_lock' }))
  }, [])

  const manualReconnect = useCallback(() => {
    reconnectAttemptsRef.current = 0
    setReconnectAttempts(0)
    const port = getApiPort()
    if (port) connectRef.current(port)
  }, [])

  const onStreamChunk = useCallback((cb: (chunk: string) => void) => {
    streamChunkCallbacksRef.current.add(cb)
    return () => { streamChunkCallbacksRef.current.delete(cb) }
  }, [])

  const onFinalAnswer = useCallback((cb: (answer: string) => void) => {
    finalAnswerCallbacksRef.current.add(cb)
    return () => { finalAnswerCallbacksRef.current.delete(cb) }
  }, [])

  const onSubAgentStart = useCallback((cb: (info: { span_id: string; agent_name: string; task: string }) => void) => {
    subAgentStartCallbacksRef.current.add(cb)
    return () => { subAgentStartCallbacksRef.current.delete(cb) }
  }, [])

  const onSubAgentStreamChunk = useCallback((cb: (span_id: string, chunk: string) => void) => {
    subAgentStreamChunkCallbacksRef.current.add(cb)
    return () => { subAgentStreamChunkCallbacksRef.current.delete(cb) }
  }, [])

  const onSubAgentEnd = useCallback((cb: (span_id: string, result: string) => void) => {
    subAgentEndCallbacksRef.current.add(cb)
    return () => { subAgentEndCallbacksRef.current.delete(cb) }
  }, [])

  const onAgentsChanged = useCallback((cb: () => void) => {
    agentsChangedCallbacksRef.current.add(cb)
    return () => { agentsChangedCallbacksRef.current.delete(cb) }
  }, [])

  const onError = useCallback((cb: (message: string) => void) => {
    errorCallbacksRef.current.add(cb)
    return () => { errorCallbacksRef.current.delete(cb) }
  }, [])

  const onSecurityEvent = useCallback((cb: (message: string) => void) => {
    securityCallbacksRef.current.add(cb)
    return () => { securityCallbacksRef.current.delete(cb) }
  }, [])

  const onPermissionRequest = useCallback((cb: (payload: PermissionRequestPayload) => void) => {
    permissionRequestCallbacksRef.current.add(cb)
    return () => { permissionRequestCallbacksRef.current.delete(cb) }
  }, [])

  const onSnapshotStatus = useCallback((cb: (status: SnapshotStatus) => void) => {
    snapshotStatusCallbacksRef.current.add(cb)
    return () => { snapshotStatusCallbacksRef.current.delete(cb) }
  }, [])

  const onSnapshotMessage = useCallback((cb: (message: SnapshotMessage) => void) => {
    snapshotMessageCallbacksRef.current.add(cb)
    return () => { snapshotMessageCallbacksRef.current.delete(cb) }
  }, [])

  return {
    connected,
    reconnecting,
    reconnectAttempts,
    isReconnectExhausted,
    processing,
    sendMessage,
    sendCancel,
    sendPermissionResponse,
    sendManualSnapshot,
    sendRestore,
    sendSystemLock,
    events,
    steps,
    subagentRuns,
    manualReconnect,
    onStreamChunk,
    onFinalAnswer,
    onSubAgentStart,
    onSubAgentStreamChunk,
    onSubAgentEnd,
    onAgentsChanged,
    onError,
    onSecurityEvent,
    onPermissionRequest,
    onSnapshotStatus,
    onSnapshotMessage,
  }
}
