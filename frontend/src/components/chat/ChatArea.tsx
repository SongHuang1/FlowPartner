import { Send } from 'lucide-react'
import { useState, useRef, useLayoutEffect, useEffect } from 'react'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'

interface Message {
  id: string
  role: 'system' | 'user-ephemeral'
  content: string
}

const WELCOME_MESSAGE: Message = {
  id: 'welcome',
  role: 'system',
  content: '你好！我是你的 AI 助手 FlowPartner，有什么需要帮忙的吗？',
}

export function MessageList({ messages }: { messages: Message[] }) {
  const scrollRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    scrollRef.current?.scrollTo({
      top: scrollRef.current.scrollHeight,
      behavior: 'smooth',
    })
  }, [messages])

  return (
    <div ref={scrollRef} className="flex flex-col gap-3 p-4 overflow-y-auto">
      {messages.map((msg) => (
        <div
          key={msg.id}
          className={`flex ${msg.role === 'user-ephemeral' ? 'justify-end' : 'justify-start'}`}
        >
          <div
            className={`max-w-[75%] rounded-lg px-4 py-2 text-sm ${
              msg.role === 'user-ephemeral'
                ? 'bg-blue-500/60 text-white'
                : 'bg-neutral-100 text-neutral-800'
            }`}
          >
            {msg.content}
          </div>
        </div>
      ))}
    </div>
  )
}

export function ChatInput({ onSend }: { onSend: (text: string) => void }) {
  const [value, setValue] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)

  const handleSend = () => {
    const trimmed = value.trim()
    if (!trimmed) return
    onSend(trimmed)
    setValue('')
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
        onChange={(e) => setValue(e.target.value)}
        onKeyDown={handleKeyDown}
        placeholder="输入消息（预览模式，暂不发送）"
        className="flex-1"
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

export function ChatArea() {
  const [messages, setMessages] = useState<Message[]>([WELCOME_MESSAGE])
  const ephemeralTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null)

  useEffect(() => {
    return () => {
      if (ephemeralTimeoutRef.current) {
        clearTimeout(ephemeralTimeoutRef.current)
      }
    }
  }, [])

  const handleSend = (text: string) => {
    if (ephemeralTimeoutRef.current) {
      clearTimeout(ephemeralTimeoutRef.current)
    }

    const ephemeralMsg: Message = {
      id: `ephemeral-${Date.now()}`,
      role: 'user-ephemeral',
      content: text,
    }
    setMessages([WELCOME_MESSAGE, ephemeralMsg])

    ephemeralTimeoutRef.current = setTimeout(() => {
      setMessages([WELCOME_MESSAGE])
      ephemeralTimeoutRef.current = null
    }, 3000)
  }

  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      <MessageList messages={messages} />
      <ChatInput onSend={handleSend} />
    </div>
  )
}
