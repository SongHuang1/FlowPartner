import { useState, useRef, useLayoutEffect, useEffect } from 'react'
import { Send, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Message } from '@/types'
import { useConversation } from '@/hooks/useConversation'
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
}

export function ChatInput({ value, onChange, onSend, disabled }: ChatInputProps) {
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
        placeholder="Type a message..."
        className="flex-1"
        maxLength={10000}
        disabled={disabled}
      />
      <Button
        size="icon"
        disabled={!value.trim() || disabled}
        onClick={handleSend}
        aria-label="Send"
      >
        {disabled ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
      </Button>
    </div>
  )
}

function LoadingSpinner() {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-sm text-neutral-400">Loading...</div>
    </div>
  )
}

function ErrorBanner({ message }: { message: string }) {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-sm text-red-500 bg-red-50 px-4 py-2 rounded-md">
        {message}
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
      ? `Thinking (${iteration}/${maxIterations})`
      : `Thinking (round ${iteration})`
    : 'Thinking...'

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

export function ChatArea() {
  const { messages, loading, error, sendMessage, addAssistantMessage } = useConversation()
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

  if (loading) return <LoadingSpinner />
  if (error) return <ErrorBanner message={error} />

  const handleSend = () => {
    const trimmed = inputValue.trim()
    if (!trimmed) return

    if (lockStatus.locked) {
      setChatError('Please unlock the API Key first')
      return
    }

    setInputValue('')
    setChatError(null)
    sendMessage(trimmed)

    const sent = wsSendMessage(trimmed)
    if (!sent) {
      if (!connected) {
        setChatError('Connecting to network, please retry later')
      } else {
        setChatError('Failed to send message, please retry')
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
          disabled={processing || lockStatus.locked}
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
            disabled={processing || lockStatus.locked}
          />
        </>
      )}
    </div>
  )
}
