import { useState, useRef, useLayoutEffect, useEffect, useCallback } from 'react'
import { Send, Square, Loader2 } from 'lucide-react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Message, PermissionRequestPayload, AgentMeta } from '@/types'
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
  const { messages, sessionId, streamingContent, subagentResults, sendMessage, appendStreamChunk, finalizeStream, addSubAgentStart, appendSubAgentChunk, finalizeSubAgent } = conversation
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
    onSubAgentStart,
    onSubAgentStreamChunk,
    onSubAgentEnd,
    onAgentsChanged,
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

  const unregisterStreamChunkRef = useRef<(() => void) | null>(null)
  const unregisterFinalAnswerRef = useRef<(() => void) | null>(null)
  const unregisterSubAgentStartRef = useRef<(() => void) | null>(null)
  const unregisterSubAgentStreamChunkRef = useRef<(() => void) | null>(null)
  const unregisterSubAgentEndRef = useRef<(() => void) | null>(null)
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
    unregisterSubAgentStartRef.current = onSubAgentStart((info) => {
      addSubAgentStart(info)
    })
    unregisterSubAgentStreamChunkRef.current = onSubAgentStreamChunk((span_id, chunk) => {
      appendSubAgentChunk(span_id, chunk)
    })
    unregisterSubAgentEndRef.current = onSubAgentEnd((span_id, result) => {
      finalizeSubAgent(span_id, result)
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
      unregisterSubAgentStartRef.current?.()
      unregisterSubAgentStreamChunkRef.current?.()
      unregisterSubAgentEndRef.current?.()
      unregisterErrorRef.current?.()
      unregisterSecurityRef.current?.()
      unregisterPermissionRef.current?.()
    }
  }, [onStreamChunk, onFinalAnswer, onSubAgentStart, onSubAgentStreamChunk, onSubAgentEnd, onError, onSecurityEvent, onPermissionRequest, appendStreamChunk, finalizeStream, addSubAgentStart, appendSubAgentChunk, finalizeSubAgent])

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
    // 显示原始消息（包含 @mention），但发送剥离后的内容给后端
    const displayContent = trimmed
    const history = sendMessage(displayContent)

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
          {subagentResults.length > 0 && (
            <div className="px-4 py-3 space-y-3 border-t border-neutral-100 bg-neutral-50/50">
              <p className="text-xs font-medium text-neutral-500">子智能体调用</p>
              {subagentResults.map((result) => (
                <div key={result.span_id} className="rounded-lg border border-neutral-200 bg-white overflow-hidden">
                  <div className="flex items-center gap-2 px-3 py-2 bg-neutral-100 border-b border-neutral-200">
                    <span className="text-sm font-medium text-neutral-700">{result.agent_name}</span>
                    <span className="text-xs text-neutral-400 truncate flex-1">{result.task}</span>
                    {result.status === 'running' && (
                      <span className="text-xs text-blue-600 flex items-center gap-1">
                        <Loader2 className="w-3 h-3 animate-spin" />
                        执行中
                      </span>
                    )}
                    {result.status === 'done' && (
                      <span className="text-xs text-green-600">已完成</span>
                    )}
                  </div>
                  {result.content && (
                    <div className="px-3 py-2 text-sm text-neutral-800 prose prose-sm max-w-none">
                      <Markdown
                        remarkPlugins={[remarkGfm, remarkMath]}
                        components={{
                          code: (props) => {
                            const { inline, className, children, ...rest } = props as { inline?: boolean; className?: string; children?: React.ReactNode; [key: string]: unknown }
                            if (className && (className.includes('math-display') || className.includes('math-inline'))) {
                              return <code className={className} {...rest}>{children}</code>
                            }
                            const hasLanguage = /language-(\w+)/.exec(className || '')
                            if (inline || !hasLanguage) {
                              return <code className="bg-neutral-100 px-1 py-0.5 rounded text-pink-600 text-[0.875em] font-mono" {...rest}>{children}</code>
                            }
                            return (
                              <div className="relative my-2">
                                {className && (
                                  <div className="absolute top-0 left-0 px-2 py-0.5 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono z-10">
                                    {className.replace('language-', '')}
                                  </div>
                                )}
                                <pre className="bg-neutral-900 text-neutral-100 pt-6 p-3 rounded-lg overflow-x-auto max-h-[300px]">
                                  <code className="font-mono text-xs leading-relaxed" {...rest}>{children}</code>
                                </pre>
                              </div>
                            )
                          },
                        }}
                      >
                        {result.content}
                      </Markdown>
                    </div>
                  )}
                </div>
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
