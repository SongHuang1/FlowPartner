import { useState, useRef, useLayoutEffect, useEffect } from 'react'
import { Send, Square, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Message, PermissionRequestPayload, AgentMeta, SubAgentRun } from '@/types'
import type { UseConversationReturn } from '@/hooks/useConversation'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { useWebSocket } from '@/hooks/useWebSocket'
import { listAgents } from '@/lib/api'
import { UserMessage } from './UserMessage'
import { AssistantMessage } from './AssistantMessage'
import { WelcomeView } from './WelcomeView'
import { ConnectionStatus } from './ConnectionStatus'
import { PermissionDialog } from './PermissionDialog'
import { AgentSelector } from './AgentSelector'
import { SubAgentCard } from './SubAgentCard'
import { SubAgentDrilldown } from './SubAgentDrilldown'

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
  const inputRef = useRef<HTMLInputElement>(null)
  const [cancelClicked, setCancelClicked] = useState(false)

  const handleSend = () => {
    const trimmed = value.trim()
    if (!trimmed || disabled) return
    setCancelClicked(false)
    onSend()
    inputRef.current?.focus()
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
    <div className="border-t border-neutral-200 p-3 flex items-center gap-2 bg-white">
      <Input
        ref={inputRef}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="输入消息..."
        className="flex-1"
        maxLength={10000}
        disabled={disabled}
      />
      {loading ? (
        <Button
          size="icon"
          variant="destructive"
          onClick={handleStop}
          disabled={cancelClicked}
          aria-label="停止"
        >
          <Square className="w-4 h-4" />
        </Button>
      ) : (
        <Button
          size="icon"
          disabled={!value.trim() || disabled}
          onClick={handleSend}
          aria-label="发送"
        >
          <Send className="w-4 h-4" />
        </Button>
      )}
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
  const { messages, sessionId, streamingContent, sendMessage, appendStreamChunk, finalizeStream } = conversation
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
    subagentRuns,
    manualReconnect,
    onStreamChunk,
    onFinalAnswer,
    onError,
    onSecurityEvent,
    onPermissionRequest,
  } = useWebSocket()

  const [inputValue, setInputValue] = useState('')
  const [chatError, setChatError] = useState<string | null>(null)
  const [securityWarning, setSecurityWarning] = useState<string | null>(null)
  const [pendingPermission, setPendingPermission] = useState<PermissionRequestPayload | null>(null)
  const [agents, setAgents] = useState<AgentMeta[]>([])
  const [executorAgentId, setExecutorAgentId] = useState('')
  const [drilldownRun, setDrilldownRun] = useState<SubAgentRun | null>(null)

  useEffect(() => {
    let cancelled = false
    listAgents()
      .then((items) => {
        if (!cancelled) setAgents(items)
      })
      .catch(() => {})
    return () => { cancelled = true }
  }, [])

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
      finalizeStream(answer)
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
  }, [onStreamChunk, onFinalAnswer, onError, onSecurityEvent, onPermissionRequest, appendStreamChunk, finalizeStream])

  const handleSend = () => {
    const trimmed = inputValue.trim()
    if (!trimmed) return

    if (lockStatus.locked) {
      setChatError('请先解锁 API Key')
      return
    }

    let injectAgentId: string | undefined
    const kept: string[] = []
    for (const token of trimmed.split(/\s+/)) {
      if (token.startsWith('@')) {
        const agent = agents.find((a) => a.name === token.slice(1))
        if (agent) {
          injectAgentId = agent.id
          continue
        }
      }
      kept.push(token)
    }
    const content = kept.join(' ')
    if (!content) {
      setChatError('消息内容为空，请补充要交给智能体执行的任务')
      return
    }
    const executor = executorAgentId || 'main'
    if (injectAgentId && injectAgentId === executor) {
      setChatError('不能强制调用会话执行者本身')
      return
    }

    setInputValue('')
    setChatError(null)
    const history = sendMessage(content)

    const sent = wsSendMessage(content, sessionId, history, executor, injectAgentId)
    if (!sent) {
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
      {drilldownRun && (
        <SubAgentDrilldown run={drilldownRun} onClose={() => setDrilldownRun(null)} />
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
            reconnectAttempts={reconnectAttempts}
            maxReconnectAttempts={5}
            reconnectExhausted={isReconnectExhausted}
            onManualReconnect={manualReconnect}
          />
          <MessageList messages={messages} streamingContent={streamingContent} />
          {processing && !streamingContent && (
            <ThinkingIndicator />
          )}
          {subagentRuns.length > 0 && (
            <div className="px-4 py-2 space-y-2 border-t border-neutral-100">
              <p className="text-xs text-neutral-400">子智能体任务</p>
              {subagentRuns.map((run) => (
                <SubAgentCard key={run.span_id} run={run} onClick={setDrilldownRun} />
              ))}
            </div>
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
            <span className="text-xs text-neutral-400">在消息中输入 @智能体名 可强制指定子智能体执行</span>
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
