import { useState, useRef, useLayoutEffect, useEffect, useCallback } from 'react'
import { Send, Square, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import type { Message, PermissionRequestPayload, AgentMeta } from '@/types'
import type { UseConversationReturn } from '@/hooks/useConversation'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { useWebSocket, deriveContentBlocks } from '@/hooks/useWebSocket'
import { listAgents } from '@/lib/api'
import { UserMessage } from './UserMessage'
import { AssistantMessage } from './AssistantMessage'
import { WelcomeView } from './WelcomeView'
import { ConnectionStatus } from './ConnectionStatus'
import { PermissionDialog } from './PermissionDialog'
import { AgentSelector } from './AgentSelector'


export function MessageList({ messages, streamingContent }: { messages: Message[]; streamingContent: string }) {
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
          return <UserMessage key={msg.id} message={msg} />
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
}

export function ChatInput({ value, onChange, onSend, onStop, disabled, loading }: ChatInputProps) {
  const textareaRef = useRef<HTMLTextAreaElement>(null)
  const [cancelClicked, setCancelClicked] = useState(false)

  const adjustHeight = useCallback(() => {
    const el = textareaRef.current
    if (!el) return
    el.style.height = 'auto'
    el.style.height = `${Math.min(el.scrollHeight, 200)}px`
  }, [])

  useEffect(() => {
    adjustHeight()
  }, [value, adjustHeight])

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
        <textarea
          ref={textareaRef}
          value={value}
          onChange={(e) => onChange(e.target.value)}
          onKeyDown={handleKeyDown}
          placeholder="输入消息..."
          maxLength={10000}
          disabled={disabled}
          rows={1}
          className="flex-1 resize-none overflow-y-auto rounded-md border border-neutral-200 bg-white px-3 py-2 text-sm placeholder:text-neutral-500 focus:outline-none focus:ring-1 focus:ring-neutral-300 disabled:opacity-50 min-h-[40px] max-h-[200px]"
        />
        {loading ? (
          <Button
            size="icon"
            variant="destructive"
            onClick={handleStop}
            disabled={cancelClicked}
            aria-label="停止"
            className="shrink-0"
          >
            <Square className="w-4 h-4" />
          </Button>
        ) : (
          <Button
            size="icon"
            disabled={!value.trim() || disabled}
            onClick={handleSend}
            aria-label="发送"
            className="shrink-0"
          >
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
  const { messages, sessionId, streamingContent, sendMessage, appendStreamChunk, finalizeWithBlocks, updateContentBlocks } = conversation
  const { settings } = useSettings()
  const { lockStatus } = useLock()
  const {
    connected,
    reconnecting,
    reconnectAttempts,
    isReconnectExhausted,
    processing,
    sendMessage: wsSendMessage,
    sendCancel,
    sendPermissionResponse,
    manualReconnect,
    onStreamChunk,
    onFinalAnswer,
    onAgentsChanged,
    onError,
    onSecurityEvent,
    onPermissionRequest,
    events,
  } = useWebSocket()

  const [inputValue, setInputValue] = useState('')
  const [chatError, setChatError] = useState<string | null>(null)
  const [securityWarning, setSecurityWarning] = useState<string | null>(null)
  const [pendingPermission, setPendingPermission] = useState<PermissionRequestPayload | null>(null)
  const [agents, setAgents] = useState<AgentMeta[]>([])
  const [executorAgentId, setExecutorAgentId] = useState('')

  const refreshAgents = useCallback(() => {
    listAgents()
      .then((items) => setAgents(items))
      .catch(() => {})
  }, [])

  useEffect(() => {
    refreshAgents()
  }, [refreshAgents])

  // 监听 agents_changed 事件，自动刷新智能体列表
  useEffect(() => {
    const unregister = onAgentsChanged(() => {
      refreshAgents()
    })
    return unregister
  }, [onAgentsChanged, refreshAgents])

  // 从事件流按时间顺序构建内容块（文本 + 子智能体卡片穿插）
  useEffect(() => {
    const blocks = deriveContentBlocks(events)
    updateContentBlocks(blocks)
  }, [events, updateContentBlocks])

  const unregisterStreamChunkRef = useRef<(() => void) | null>(null)
  const unregisterFinalAnswerRef = useRef<(() => void) | null>(null)
  const unregisterErrorRef = useRef<(() => void) | null>(null)
  const unregisterSecurityRef = useRef<(() => void) | null>(null)
  const unregisterPermissionRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    unregisterStreamChunkRef.current = onStreamChunk((chunk) => {
      appendStreamChunk(chunk)
    })
    unregisterFinalAnswerRef.current = onFinalAnswer((answer) => {
      const blocks = deriveContentBlocks(events)
      finalizeWithBlocks(answer, blocks)
    })
    unregisterErrorRef.current = onError((message) => {
      setChatError(message)
    })
    unregisterSecurityRef.current = onSecurityEvent((message) => {
      setSecurityWarning(message)
    })
    unregisterPermissionRef.current = onPermissionRequest((payload) => {
      setPendingPermission(payload)
    })

    return () => {
      unregisterStreamChunkRef.current?.()
      unregisterFinalAnswerRef.current?.()
      unregisterErrorRef.current?.()
      unregisterSecurityRef.current?.()
      unregisterPermissionRef.current?.()
    }
  }, [onStreamChunk, onFinalAnswer, onError, onSecurityEvent, onPermissionRequest, appendStreamChunk, finalizeWithBlocks, events])

  const handleSend = () => {
    const trimmed = inputValue.trim()
    if (!trimmed) return

    if (lockStatus.locked) {
      setChatError('请先解锁 API Key')
      return
    }

    setInputValue('')
    setChatError(null)
    const history = sendMessage(trimmed)

    const executor = executorAgentId || 'main'
    console.log('[ChatArea] Sending message:', { content: trimmed, sessionId, executor, historyLen: history.length })
    const sent = wsSendMessage(trimmed, sessionId, history, executor)
    if (!sent) {
      console.error('[ChatArea] Failed to send message via WebSocket')
      if (!connected) {
        setChatError('网络连接中，请稍后重试')
      } else {
        setChatError('消息发送失败，请重试')
      }
    }
  }

  const handleStop = () => {
    sendCancel(sessionId)
  }

  const handlePermissionDecision = (decision: 'allow' | 'allow_session' | 'deny') => {
    if (pendingPermission) {
      if (decision === 'allow_session') {
        sendPermissionResponse(sessionId, pendingPermission.request_id, 'allow', 'session')
      } else {
        sendPermissionResponse(sessionId, pendingPermission.request_id, decision)
      }
      setPendingPermission(null)
    }
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {pendingPermission && (
        <PermissionDialog
          request={pendingPermission}
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
          onAgentsChanged={onAgentsChanged}
        />
      ) : (
        <>
          <ConnectionStatus
            connected={connected}
            reconnecting={reconnecting}
            reconnectAttempts={reconnectAttempts}
            maxReconnectAttempts={5}
            reconnectExhausted={isReconnectExhausted}
            onManualReconnect={manualReconnect}
          />

          <MessageList messages={messages} streamingContent={streamingContent} />
          {processing && !streamingContent && (
            <ThinkingIndicator />
          )}
          {securityWarning && (
            <div className="px-4 py-2 text-sm text-amber-800 bg-amber-50 border-t border-amber-200">
              {securityWarning}
            </div>
          )}
          {chatError && (
            <div className="px-4 py-2 text-sm text-red-500 bg-red-50">
              {chatError}
            </div>
          )}
          <div className="flex items-center gap-3 px-4 pt-2">
            <AgentSelector
              agents={agents}
              value={executorAgentId}
              onChange={setExecutorAgentId}
              disabled={lockStatus.locked || processing}
            />
            <span className="text-xs text-neutral-400">在消息中输入 @智能体名 可建议主智能体调用子智能体</span>
          </div>
          <ChatInput
            value={inputValue}
            onChange={setInputValue}
            onSend={handleSend}
            onStop={handleStop}
            disabled={lockStatus.locked}
            loading={processing}
          />
        </>
      )}
    </div>
  )
}
