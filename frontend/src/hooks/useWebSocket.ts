import { useCallback, useEffect, useRef, useState } from 'react'

const MAX_RECONNECT_ATTEMPTS = 5
const RECONNECT_INTERVAL_MS = 3000
const PORT_MIN = 1024
const PORT_MAX = 65535

type ConnectionState = 'disconnected' | 'connecting' | 'connected' | 'reconnecting' | 'reconnect_exhausted'

interface WsEnvelope {
  id?: string | number
  method?: string
  params?: unknown
  result?: unknown
  error?: { code: number; message: string; data?: unknown }
}

export interface ApprovalRequestPayload {
  threadId: string
  requestId: number
  command?: string
  tool?: string
  path?: string
  operation?: string
  detail?: string
  availableDecisions: string[]
}

interface WsV2Callbacks {
  onRequestApproval?: (payload: ApprovalRequestPayload) => void
  onServerRequestResolved?: (payload: { threadId: string; requestId: number }) => void
  onThreadEvent?: (method: string, params: unknown) => void
  onGlobalEvent?: (eventType: string, payload: string) => void
}

export function useWsV2(callbacks: WsV2Callbacks) {
  const [connectionState, setConnectionState] = useState<ConnectionState>('disconnected')
  const wsRef = useRef<WebSocket | null>(null)
  const requestIdRef = useRef(0)
  const pendingReqsRef = useRef<Map<string, { resolve: (v: unknown) => void; reject: (e: Error) => void }>>(new Map())
  const reconnectAttemptsRef = useRef(0)
  const reconnectTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const shouldReconnectRef = useRef(true)
  const mountedRef = useRef(false)
  const callbacksRef = useRef(callbacks)
  const apiRef = useRef<{
    connect: () => void
    disconnect: () => void
    handshake: () => void
    attemptReconnect: () => void
    handleEnvelope: (env: WsEnvelope) => void
    handleLegacyMessage: (data: string) => void
  } | null>(null)

  useEffect(() => {
    callbacksRef.current = callbacks
  })

  const sendRequest = useCallback(async (method: string, params?: unknown): Promise<unknown> => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) {
      throw new Error('WebSocket 未连接')
    }
    const id = `req-${++requestIdRef.current}`
    const envelope: WsEnvelope = { id, method, params: params ?? {} }
    return new Promise((resolve, reject) => {
      pendingReqsRef.current.set(id, { resolve, reject })
      ws.send(JSON.stringify(envelope))
      setTimeout(() => {
        if (pendingReqsRef.current.has(id)) {
          pendingReqsRef.current.delete(id)
          reject(new Error(`请求超时: ${method}`))
        }
      }, 30000)
    })
  }, [])

  const sendNotification = useCallback((method: string, params?: unknown) => {
    const ws = wsRef.current
    if (!ws || ws.readyState !== WebSocket.OPEN) return
    ws.send(JSON.stringify({ method, params: params ?? {} }))
  }, [])

  useEffect(() => {
    mountedRef.current = true

    const handleEnvelope = (env: WsEnvelope) => {
      if (env.id !== undefined && pendingReqsRef.current.has(String(env.id))) {
        const { resolve, reject } = pendingReqsRef.current.get(String(env.id))!
        pendingReqsRef.current.delete(String(env.id))
        if (env.error) {
          reject(new Error(env.error.message))
        } else {
          resolve(env.result)
        }
        return
      }
      if (env.error) return

      if (env.method) {
        const params = env.params as Record<string, unknown> | undefined

        if (env.method === 'item/commandExecution/requestApproval') {
          const p = params as { threadId?: string; requestId?: number; command?: string; tool?: string; path?: string; operation?: string; detail?: string; availableDecisions?: string[] }
          callbacksRef.current.onRequestApproval?.({
            threadId: p.threadId ?? '',
            requestId: p.requestId ?? 0,
            command: p.command,
            tool: p.tool,
            path: p.path,
            operation: p.operation,
            detail: p.detail,
            availableDecisions: p.availableDecisions ?? ['approved', 'denied'],
          })
          return
        }

        if (env.method === 'serverRequest/resolved') {
          const p = params as { threadId?: string; requestId?: number }
          callbacksRef.current.onServerRequestResolved?.({
            threadId: p.threadId ?? '',
            requestId: p.requestId ?? 0,
          })
          return
        }

        callbacksRef.current.onThreadEvent?.(env.method, env.params)
      }
    }

    const handleLegacyMessage = (data: string) => {
      try {
        const msg = JSON.parse(data)
        if (msg.event_type) {
          callbacksRef.current.onGlobalEvent?.(msg.event_type, msg.payload)
        }
      } catch { /* ignore */ }
    }

    const handshake = async () => {
      try {
        await sendRequest('initialize', {
          clientInfo: {
            name: 'FlowPartner',
            title: 'FlowPartner',
            version: window.flowPartner.getVersion?.() || '0.3.0',
          },
        })
        sendNotification('initialized')
      } catch {
        wsRef.current?.close()
      }
    }

    const attemptReconnect = () => {
      if (!mountedRef.current) return
      if (reconnectAttemptsRef.current >= MAX_RECONNECT_ATTEMPTS) {
        setConnectionState('reconnect_exhausted')
        return
      }
      setConnectionState('reconnecting')
      reconnectAttemptsRef.current++
      reconnectTimerRef.current = setTimeout(() => {
        doConnect()
      }, RECONNECT_INTERVAL_MS)
    }

    const doConnect = async () => {
      if (!mountedRef.current) return
      if (wsRef.current?.readyState === WebSocket.OPEN) return

      setConnectionState('connecting')
      shouldReconnectRef.current = true

      try {
        const port = await window.flowPartner.fetchBackendPort()
        if (typeof port !== 'number' || isNaN(port) || port < PORT_MIN || port > PORT_MAX) {
          throw new Error(`无效端口: ${port}`)
        }

        const ws = new WebSocket(`ws://localhost:${port}/ws`)
        wsRef.current = ws

        ws.onopen = () => {
          reconnectAttemptsRef.current = 0
          setConnectionState('connected')
          handshake()
        }

        ws.onmessage = (event) => {
          try {
            const env: WsEnvelope = JSON.parse(event.data)
            handleEnvelope(env)
          } catch {
            handleLegacyMessage(event.data)
          }
        }

        ws.onclose = () => {
          wsRef.current = null
          pendingReqsRef.current.forEach(({ reject }) => reject(new Error('连接已断开')))
          pendingReqsRef.current.clear()
          if (shouldReconnectRef.current && mountedRef.current) {
            attemptReconnect()
          } else {
            setConnectionState('disconnected')
          }
        }

        ws.onerror = () => {
          ws.close()
        }
      } catch {
        setConnectionState('disconnected')
      }
    }

    const doDisconnect = () => {
      shouldReconnectRef.current = false
      if (reconnectTimerRef.current) {
        clearTimeout(reconnectTimerRef.current)
        reconnectTimerRef.current = null
      }
      wsRef.current?.close()
      wsRef.current = null
      setConnectionState('disconnected')
    }

    apiRef.current = {
      connect: doConnect,
      disconnect: doDisconnect,
      handshake,
      attemptReconnect,
      handleEnvelope,
      handleLegacyMessage,
    }

    doConnect()

    return () => {
      mountedRef.current = false
      doDisconnect()
    }
  }, [sendRequest, sendNotification])

  const connect = useCallback(() => {
    apiRef.current?.connect()
  }, [])

  const disconnect = useCallback(() => {
    apiRef.current?.disconnect()
  }, [])

  const startChat = useCallback(async (params: {
    threadId?: string
    input: { type: string; text: string }[]
    cwd?: string
    agentId?: string
  }) => {
    const result = await sendRequest('turn/start', {
      threadId: params.threadId,
      input: params.input,
      overrides: {
        cwd: params.cwd,
        agentId: params.agentId,
      },
    })
    return result as { threadId: string; turnId: string }
  }, [sendRequest])

  const steer = useCallback(async (params: {
    threadId: string
    turnId?: string
    input: { type: string; text: string }[]
  }) => {
    return sendRequest('turn/steer', params)
  }, [sendRequest])

  const interrupt = useCallback(async (params: { threadId: string; turnId?: string }) => {
    return sendRequest('turn/interrupt', params)
  }, [sendRequest])

  const respondToApproval = useCallback(async (params: {
    threadId: string
    requestId: number
    decision: string
  }) => {
    return sendRequest('response/' + params.requestId, {
      threadId: params.threadId,
      decision: params.decision,
    })
  }, [sendRequest])

  const listThreads = useCallback(async (params?: { cursor?: string; limit?: number; archived?: boolean }) => {
    return sendRequest('thread/list', params ?? {})
  }, [sendRequest])

  const readThread = useCallback(async (threadId: string) => {
    return sendRequest('thread/read', { threadId })
  }, [sendRequest])

  const startThread = useCallback(async (params?: { cwd?: string; agentId?: string }) => {
    return sendRequest('thread/start', params ?? {})
  }, [sendRequest])

  const triggerSnapshot = useCallback(async () => {
    return sendRequest('snapshot/trigger')
  }, [sendRequest])

  const restoreSnapshot = useCallback(async (snapshotId: string, deleteExtras: boolean) => {
    return sendRequest('snapshot/restore', { snapshotId, deleteExtras })
  }, [sendRequest])

  const systemLock = useCallback(async () => {
    return sendRequest('system/lock')
  }, [sendRequest])

  return {
    connectionState,
    sendRequest,
    sendNotification,
    connect,
    disconnect,
    startChat,
    steer,
    interrupt,
    respondToApproval,
    listThreads,
    readThread,
    startThread,
    triggerSnapshot,
    restoreSnapshot,
    systemLock,
  }
}

export type WsV2Hook = ReturnType<typeof useWsV2>
