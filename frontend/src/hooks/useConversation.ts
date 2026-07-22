import { useState, useEffect, useRef } from 'react'
import type { Message, Conversation } from '@/types'
import { getConversation, saveConversation } from '@/lib/api'

function generateMessageId(): string {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  const array = new Uint8Array(6)
  crypto.getRandomValues(array)
  return `msg_${Date.now()}_${Array.from(array, b => chars[b % chars.length]).join('')}`
}

interface UseConversationReturn {
  messages: Message[]
  loading: boolean
  error: string | null
  sendMessage: (content: string) => void
}

export function useConversation(): UseConversationReturn {
  const [messages, setMessages] = useState<Message[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const messagesRef = useRef<Message[]>([])
  const loadedRef = useRef(false)

  useEffect(() => {
    getConversation()
      .then((conv: Conversation) => {
        messagesRef.current = conv.messages
        setMessages(conv.messages)
      })
      .catch((e: Error) => setError(e.message))
      .finally(() => {
        loadedRef.current = true
        setLoading(false)
      })
  }, [])

  useEffect(() => {
    if (!loadedRef.current) return
    saveConversation(messages).catch((e: Error) => setError(`保存聊天记录失败: ${e.message}`))
  }, [messages])

  const sendMessage = (content: string) => {
    const trimmed = content.trim()
    if (!trimmed) return

    const newMessage: Message = {
      id: generateMessageId(),
      role: 'user',
      content: trimmed,
      timestamp: Date.now(),
    }

    const updated = [...messagesRef.current, newMessage]
    messagesRef.current = updated
    setMessages(updated)
  }

  return { messages, loading, error, sendMessage }
}
