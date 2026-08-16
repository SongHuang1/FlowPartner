import { useState, useRef, useCallback } from 'react'
import type { Message } from '@/types'

function generateMessageId(): string {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  const array = new Uint8Array(6)
  crypto.getRandomValues(array)
  return `msg_${Date.now()}_${Array.from(array, b => chars[b % chars.length]).join('')}`
}

function generateSessionId(): string {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  const array = new Uint8Array(8)
  crypto.getRandomValues(array)
  return `sess_${Date.now()}_${Array.from(array, b => chars[b % chars.length]).join('')}`
}

export interface UseConversationReturn {
  messages: Message[]
  sessionId: string
  sendMessage: (content: string) => Message[]
  addAssistantMessage: (content: string) => void
  startNewConversation: () => void
  loadConversation: (sessionId: string, messages: Message[]) => void
}

export function useConversation(): UseConversationReturn {
  // 启动时始终为空对话（欢迎页），历史会话通过“查看历史”手动加载
  const [messages, setMessages] = useState<Message[]>([])
  const [sessionId, setSessionId] = useState<string>(() => generateSessionId())
  const messagesRef = useRef<Message[]>([])

  const sendMessage = useCallback((content: string): Message[] => {
    const trimmed = content.trim()
    if (!trimmed) return []

    // 返回当前消息之前的历史，供 WebSocket 一并发送给后端
    const history = [...messagesRef.current]
    const newMessage: Message = {
      id: generateMessageId(),
      role: 'user',
      content: trimmed,
      timestamp: Date.now(),
    }
    const updated = [...messagesRef.current, newMessage]
    messagesRef.current = updated
    setMessages(updated)
    return history
  }, [])

  const addAssistantMessage = useCallback((content: string) => {
    const newMessage: Message = {
      id: generateMessageId(),
      role: 'assistant',
      content,
      timestamp: Date.now(),
    }
    const updated = [...messagesRef.current, newMessage]
    messagesRef.current = updated
    setMessages(updated)
  }, [])

  const startNewConversation = useCallback(() => {
    messagesRef.current = []
    setMessages([])
    setSessionId(generateSessionId())
  }, [])

  const loadConversation = useCallback((sid: string, msgs: Message[]) => {
    messagesRef.current = msgs
    setMessages(msgs)
    setSessionId(sid)
  }, [])

  return { messages, sessionId, sendMessage, addAssistantMessage, startNewConversation, loadConversation }
}