import { useState, useRef, useLayoutEffect, useEffect, useCallback, useMemo } from 'react'
import { Send, Square, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Message, AgentMeta } from '@/types'
import type { UseConversationReturn } from '@/hooks/useConversation'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { useWsV2, type ApprovalRequestPayload as WsApprovalRequest } from '@/hooks/useWebSocket'
import { listAgents } from '@/lib/api'
import { UserMessage } from './UserMessage'
import { AssistantMessage } from './AssistantMessage'
import { WelcomeView } from './WelcomeView'
import { ConnectionStatus } from './ConnectionStatus'
import { PermissionDialog } from './PermissionDialog'
import { AgentSelector } from './AgentSelector'
import { MentionTextarea } from './MentionTextarea'

export function MessageList({ messages, streamingContent, agentNames }: { messages: Message[]; streamingContent: string; agentNames: Set<string> }) {
  const scrollRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    scrollRef.current?.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: 'smooth',
    })
  }, [messages, streamingContent])

  return (
    <div ref={scrollRef} className="flex flex-col gap-3 p-4 overflow-y-auto flex-1">
      {messages.map((msg) => {
        if (msg.role === 'user') {
          return <UserMessage key={msg.id} message={msg} agentNames={agentNames} />
        }
        const isStreaming = msg.status === 'streaming'
        return (
          <AssistantMessage
            key={msg.id}
            message={msg}
            streamingContent={isStreaming ? streamingContent : undefined}
          />
        )
      })}
    </div>
  )
}

interface ChatInputProps {
  value: string
  onChange: (v: string) => void
  onSend: () => void
  onStop?: () => void
  disabled?: boolean
  loading?: boolean
  agents?: AgentMeta[]
}

export function ChatInput({ value, onChange, onSend, onStop, disabled, loading, agents = [] }: ChatInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement | null>(null)
  const [cancelClicked, setCancelClicked] = useState(false)

  const adjustHeight = useCallback(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`
  }, [])

  useEffect(() => { adjustHeight() }, [value, adjustHeight])

  const handleSend = () => {
    const trimmed = value.trim()
    if (!trimmed || disabled) return
    setCancelClicked(false)
    onSend()
    if (textareaRef.current) {
      textareaRef.current.style.height = 'auto'
      textareaRef.current.focus()
    }
  }

  const handleStop = () => {
    setCancelClicked(true)
    onStop?.()
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      handleSend()
    }
  }

  return (
    <div className="border-t border-neutral-200 p-3 bg-white">
      <div className="flex items-end gap-2">
        <MentionTextarea
          inputRef={textareaRef}
          value={value}
          onChange={onChange}
          onKeyDown={handleKeyDown}
          agents={agents}
          disabled={disabled}
          placeholder="输入消息..."
          maxLength={10000}
          rows={1}
        />
        {loading ? (
          <Button size="icon" variant="destructive" onClick={handleStop} disabled={cancelClicked} aria-label="停止" className="shrink-0">
            <Square className="w-4 h-4" />
          </Button>
        ) : (
          <Button size="icon" disabled={!value.trim() || disabled} onClick={handleSend} aria-label="发送" className="shrink-0">
            <Send className="w-4 h-4" />
          </Button>
        )}
      </div>
    </div>
  )
}

interface ThinkingIndicatorProps {
  iteration?: number
  maxIterations?: number
}

function ThinkingIndicator({ iteration = 0, maxIterations }: ThinkingIndicatorProps) {
  const label = iteration > 0
    ? maxIterations
      ? `思考中 (${iteration}/${maxIterations})`
      : `思考中（第 ${iteration} 轮）`
    : '思考中...'

  return (
    <div className="flex items-center gap-2 p-4 text-sm text-neutral-500">
      <Loader2 className="w-4 h-4 animate-spin" />
      <span>{label}</span>
    </div>
  )
}

interface ChatAreaProps {
  conversation: UseConversationReturn
}

export function ChatArea({ conversation }: ChatAreaProps) {
  const { messages, streamingContent, sendMessage, appendStreamChunk, finalizeStream } = conversation
  const { settings } = useSettings()
  const { lockStatus } = useLock()

  const [inputValue, setInputValue] = useState('')
  const [chatError, setChatError] = useState<string | null>(null)
  const [pendingApproval, setPendingApproval] = useState<WsApprovalRequest | null>(null)
  const [agents, setAgents] = useState<AgentMeta[]>([])
  const [executorAgentId, setExecutorAgentId] = useState('')
  const agentNames = useMemo(() => new Set(agents.map((a) => a.name)), [agents])

  const currentThreadIdRef = useRef<string>('')
  const currentTurnIdRef = useRef<string>('')
  const [processing, setProcessing] = useState(false)

  const refreshAgents = useCallback(() => {
    listAgents().then(setAgents).catch(() => {})
  }, [])

  useEffect(() => { refreshAgents() }, [refreshAgents])

  const handleThreadEvent = useCallback((method: string, params: unknown) => {
    const p = params as Record<string, unknown> | undefined
    switch (method) {
      case 'turn/started':
        setProcessing(true)
        currentTurnIdRef.current = (p as { turnId?: string })?.turnId || ''
        break
      case 'item/agentMessage/delta': {
        const delta = (p as { delta?: string })?.delta
        if (typeof delta === 'string') appendStreamChunk(delta)
        break
      }
      case 'item/completed': {
        const text = (p as { item?: { text?: string } })?.item?.text
        if (typeof text === 'string') finalizeStream(text)
        setProcessing(false)
        break
      }
      case 'turn/completed':
      case 'turn/interrupted':
        setProcessing(false)
        finalizeStream()
        break
      case 'error': {
        const msg = (p as { message?: string })?.message
        if (msg) setChatError(msg)
        setProcessing(false)
        break
      }
    }
  }, [appendStreamChunk, finalizeStream])

  const handleGlobalEvent = useCallback((eventType: string, _payload: string) => {
    if (eventType === 'agents_changed') refreshAgents()
  }, [refreshAgents])

  const handleRequestApproval = useCallback((payload: WsApprovalRequest) => {
    setPendingApproval(payload)
  }, [])

  const { connectionState, connect, startChat, interrupt, respondToApproval } = useWsV2({
    onThreadEvent: handleThreadEvent,
    onGlobalEvent: handleGlobalEvent,
    onRequestApproval: handleRequestApproval,
  })

  const connected = connectionState === 'connected'
  const reconnecting = connectionState === 'reconnecting'
  const isReconnectExhausted = connectionState === 'reconnect_exhausted'

  const handleSend = useCallback(async () => {
    const trimmed = inputValue.trim()
    if (!trimmed) return

    if (lockStatus.locked) {
      setChatError('请先解锁 API Key')
      return
    }

    setInputValue('')
    setChatError(null)
    sendMessage(trimmed)

    try {
      const result = await startChat({
        input: [{ type: 'text', text: trimmed }],
        agentId: executorAgentId || undefined,
      })
      currentThreadIdRef.current = result.threadId
    } catch (e) {
      setChatError(e instanceof Error ? e.message : '消息发送失败')
    }
  }, [inputValue, lockStatus.locked, sendMessage, startChat, executorAgentId])

  const handleStop = useCallback(async () => {
    if (currentThreadIdRef.current) {
      try {
        await interrupt({ threadId: currentThreadIdRef.current, turnId: currentTurnIdRef.current || undefined })
      } catch { /* ignore */ }
    }
  }, [interrupt])

  const handlePermissionDecision = useCallback(async (decision: string) => {
    if (pendingApproval) {
      try {
        await respondToApproval({
          threadId: pendingApproval.threadId,
          requestId: pendingApproval.requestId,
          decision,
        })
      } catch { /* ignore */ }
      setPendingApproval(null)
    }
  }, [pendingApproval, respondToApproval])

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {pendingApproval && (
        <PermissionDialog
          request={{
            request_id: String(pendingApproval.requestId),
            tool: pendingApproval.tool || 'bash',
            path: pendingApproval.path || '',
            operation: pendingApproval.operation || 'execute',
            detail: pendingApproval.detail || '',
            scope_options: pendingApproval.availableDecisions,
          }}
          onDecision={handlePermissionDecision}
        />
      )}
      {messages.length === 0 ? (
        <WelcomeView
          settings={settings}
          inputValue={inputValue}
          onInputChange={setInputValue}
          onSend={handleSend}
          disabled={lockStatus.locked}
          loading={processing}
          executorAgentId={executorAgentId}
          onExecutorChange={setExecutorAgentId}
        />
      ) : (
        <>
          <ConnectionStatus
            connected={connected}
            reconnecting={reconnecting}
            reconnectAttempts={0}
            maxReconnectAttempts={5}
            reconnectExhausted={isReconnectExhausted}
            onManualReconnect={connect}
          />

          <MessageList messages={messages} streamingContent={streamingContent} agentNames={agentNames} />
          {processing && !streamingContent && <ThinkingIndicator />}
          {chatError && (
            <div className="px-4 py-2 text-sm text-red-500 bg-red-50">{chatError}</div>
          )}
          <div className="flex items-center gap-3 px-4 pt-2">
            <AgentSelector
              agents={agents}
              value={executorAgentId}
              onChange={setExecutorAgentId}
              disabled={lockStatus.locked || processing}
            />
            <span className="text-xs text-neutral-400">输入 @ 可唤起智能体补全</span>
          </div>
          <ChatInput
            value={inputValue}
            onChange={setInputValue}
            onSend={handleSend}
            onStop={handleStop}
            disabled={lockStatus.locked}
            loading={processing}
            agents={agents}
          />
        </>
      )}
    </div>
  )
}
