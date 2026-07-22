import { useState, useRef, useLayoutEffect } from 'react'
import { Send } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import type { Message } from '@/types'
import { useConversation } from '@/hooks/useConversation'
import { useSettings } from '@/hooks/useSettings'
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
}

export function ChatInput({ value, onChange, onSend }: ChatInputProps) {
  const inputRef = useRef<HTMLInputElement>(null)

  const handleSend = () => {
    const trimmed = value.trim()
    if (!trimmed) return
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
      />
      <Button
        size="icon"
        disabled={!value.trim()}
        onClick={handleSend}
        aria-label="发送"
      >
        <Send className="w-4 h-4" />
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

export function ChatArea() {
  const { messages, loading, error, sendMessage } = useConversation()
  const { settings } = useSettings()
  const [inputValue, setInputValue] = useState('')

  if (loading) return <LoadingSpinner />
  if (error) return <ErrorBanner message={error} />

  const handleSend = () => {
    sendMessage(inputValue)
    setInputValue('')
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {messages.length === 0 ? (
        <WelcomeView
          settings={settings}
          inputValue={inputValue}
          onInputChange={setInputValue}
          onSend={handleSend}
        />
      ) : (
        <>
          <MessageList messages={messages} />
          <ChatInput
            value={inputValue}
            onChange={setInputValue}
            onSend={handleSend}
          />
        </>
      )}
    </div>
  )
}
