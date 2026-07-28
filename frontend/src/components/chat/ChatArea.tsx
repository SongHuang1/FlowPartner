import { useState, useRef, useLayoutEffect } from 'react'
import { Send, Loader2 } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Message } from '@/types'
import { useConversation } from '@/hooks/useConversation'
import { useSettings } from '@/hooks/useSettings'
import { useLock } from '@/hooks/useLock'
import { sendMessage as apiSendMessage } from '@/lib/api'
import { MessageBubble } from './MessageBubble'
import { WelcomeView } from './WelcomeView'

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
        {disabled ? <Loader2 className="w-4 h-4 animate-spin" /> : <Send className="w-4 h-4" />}
      </Button>
    </div>
  )
}

function LoadingSpinner() {
  return (
    <div className="flex-1 flex items-center justify-center">
      <div className="text-sm text-neutral-400">加载中...</div>
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

function ThinkingIndicator() {
  return (
    <div className="flex items-center gap-2 p-4 text-sm text-neutral-500">
      <Loader2 className="w-4 h-4 animate-spin" />
      <span>思考中...</span>
    </div>
  )
}

export function ChatArea() {
  const { messages, loading, error, sendMessage, addAssistantMessage } = useConversation()
  const { settings } = useSettings()
  const { lockStatus } = useLock()
  const [inputValue, setInputValue] = useState('')
  const [thinking, setThinking] = useState(false)
  const [chatError, setChatError] = useState<string | null>(null)

  if (loading) return <LoadingSpinner />
  if (error) return <ErrorBanner message={error} />

  const handleSend = async () => {
    const trimmed = inputValue.trim()
    if (!trimmed) return

    if (lockStatus.locked) {
      setChatError('请先解锁 API Key')
      return
    }

    setInputValue('')
    setChatError(null)
    sendMessage(trimmed)

    setThinking(true)
    try {
      const response = await apiSendMessage(trimmed)
      addAssistantMessage(response.content)
    } catch (e) {
      const msg = e instanceof Error ? e.message : '发送失败'
      setChatError(msg)
    } finally {
      setThinking(false)
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
          disabled={thinking || lockStatus.locked}
        />
      ) : (
        <>
          <MessageList messages={messages} />
          {thinking && <ThinkingIndicator />}
          {chatError && (
            <div className="px-4 py-2 text-sm text-red-500 bg-red-50">
              {chatError}
            </div>
          )}
          <ChatInput
            value={inputValue}
            onChange={setInputValue}
            onSend={handleSend}
            disabled={thinking}
          />
        </>
      )}
    </div>
  )
}
