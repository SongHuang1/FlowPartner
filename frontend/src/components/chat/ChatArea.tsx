import { useState, useRef, useLayoutEffect, useEffect } from 'react'
import { Send, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Message } from '@/types'
import type { UseConversationReturn } from '@/hooks/useConversation'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { useWebSocket } from '@/hooks/useWebSocket'
import { MessageBubble } from './MessageBubble'
import { WelcomeView } from './WelcomeView'
import { EventDetail } from './EventDetail'
import { ConnectionStatus } from './ConnectionStatus'

export function MessageList({ messages }: { messages: Message[] }) {
  const scrollRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    scrollRef.current?.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: 'smooth',
    })
  }, [messages])

  return (
    <div ref={scrollRef} className="flex flex-col gap-3 p-4 overflow-y-auto flex-1">
      {messages.map((msg) => (
        <MessageBubble key={msg.id} message={msg} />
      ))}
    </div>
  )
}

interface ChatInputProps {
  value: string
  onChange: (v: string) => void
  onSend: () => void
  disabled?: boolean
  loading?: boolean
}

export function ChatInput({ value, onChange, onSend, disabled, loading }: ChatInputProps) {
  const inputRef = useRef<HTMLInputElement>(null)

  const handleSend = () => {
    const trimmed = value.trim()
    if (!trimmed || disabled) return
    onSend()
    inputRef.current?.focus()
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
      <Button
        size="icon"
        disabled={!value.trim() || disabled}
        onClick={handleSend}
        aria-label="发送"
      >
        {loading ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
      </Button>
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

function EventList({ events }: { events: import('@/hooks/useWebSocket').ChatEvent[] }) {
  return (
    <div className="px-4 py-2 space-y-2 border-t border-neutral-100">
      {events.map((evt, i) => (
        <EventDetail key={i} eventType={evt.event_type} payload={evt.payload} />
      ))}
    </div>
  )
}

interface ChatAreaProps {
  conversation: UseConversationReturn
}

export function ChatArea({ conversation }: ChatAreaProps) {
  const { messages, sessionId, sendMessage, addAssistantMessage } = conversation
  const { settings } = useSettings()
  const { lockStatus } = useLock()
  const {
    connected,
    reconnecting,
    reconnectAttempts,
    isReconnectExhausted,
    processing,
    sendMessage: wsSendMessage,
    events,
    manualReconnect,
    onFinalAnswer,
    onError,
    onSecurityEvent,
  } = useWebSocket()

  const [inputValue, setInputValue] = useState('')
  const [chatError, setChatError] = useState<string | null>(null)
  const [securityWarning, setSecurityWarning] = useState<string | null>(null)

  const unregisterFinalAnswerRef = useRef<(() => void) | null>(null)
  const unregisterErrorRef = useRef<(() => void) | null>(null)
  const unregisterSecurityRef = useRef<(() => void) | null>(null)

  useEffect(() => {
    unregisterFinalAnswerRef.current = onFinalAnswer((answer) => {
      addAssistantMessage(answer)
    })
    unregisterErrorRef.current = onError((message) => {
      setChatError(message)
    })
    unregisterSecurityRef.current = onSecurityEvent((message) => {
      setSecurityWarning(message)
    })

    return () => {
      unregisterFinalAnswerRef.current?.()
      unregisterErrorRef.current?.()
      unregisterSecurityRef.current?.()
    }
  }, [onFinalAnswer, onError, onSecurityEvent, addAssistantMessage])

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

    const sent = wsSendMessage(trimmed, sessionId, history)
    if (!sent) {
      if (!connected) {
        setChatError('网络连接中，请稍后重试')
      } else {
        setChatError('消息发送失败，请重试')
      }
    }
  }

  const latestStatusUpdate = [...events]
    .reverse()
    .find((e) => e.event_type === 'status_update')
  let iteration = 0
  if (latestStatusUpdate) {
    try {
      const parsed = JSON.parse(latestStatusUpdate.payload) as { iteration?: number }
      iteration = parsed.iteration ?? 0
    } catch {
      /* malformed payload, use default iteration = 0 */
    }
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {messages.length === 0 ? (
        <WelcomeView
          settings={settings}
          inputValue={inputValue}
          onInputChange={setInputValue}
          onSend={handleSend}
          disabled={lockStatus.locked}
          loading={processing}
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
          <MessageList messages={messages} />
          {processing && <ThinkingIndicator iteration={iteration} />}
          {events.length > 0 && <EventList events={events} />}
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
          <ChatInput
            value={inputValue}
            onChange={setInputValue}
            onSend={handleSend}
            disabled={lockStatus.locked}
            loading={processing}
          />
        </>
      )}
    </div>
  )
}
