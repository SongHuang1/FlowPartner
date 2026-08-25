import { useState, useRef, useCallback } from 'react'
import type { Message, ContentBlock } from '@/types'

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
  streamingContent: string
  sendMessage: (content: string) => Message[]
  addAssistantMessage: (content: string) => void
  appendStreamChunk: (chunk: string) => void
  finalizeStream: (finalContent?: string) => void
  finalizeWithBlocks: (finalContent: string, blocks: ContentBlock[]) => void
  updateContentBlocks: (blocks: ContentBlock[]) => void
  startNewConversation: () => void
  loadConversation: (sessionId: string, messages: Message[]) => void
}

export function useConversation(): UseConversationReturn {
  const [messages, setMessages] = useState<Message[]>([])
  const [sessionId, setSessionId] = useState<string>(() => generateSessionId())
  const [streamingContent, setStreamingContent] = useState('')
  const messagesRef = useRef<Message[]>([])
  const streamingIdRef = useRef<string | null>(null)
  const streamingContentRef = useRef<string>('')

  const sendMessage = useCallback((content: string): Message[] => {
    const trimmed = content.trim()
    if (!trimmed) return []

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
      status: 'completed',
    }
    const updated = [...messagesRef.current, newMessage]
    messagesRef.current = updated
    setMessages(updated)
    setStreamingContent('')
    streamingIdRef.current = null
  }, [])

  const appendStreamChunk = useCallback((chunk: string) => {
    if (!streamingIdRef.current) {
      const id = generateMessageId()
      streamingIdRef.current = id
      const newMessage: Message = {
        id,
        role: 'assistant',
        content: '',
        timestamp: Date.now(),
        status: 'streaming',
      }
      const updated = [...messagesRef.current, newMessage]
      messagesRef.current = updated
      setMessages(updated)
      streamingContentRef.current = ''
    }
    streamingContentRef.current += chunk
    setStreamingContent(streamingContentRef.current)
  }, [])

  const finalizeStream = useCallback((finalContent?: string) => {
    if (streamingIdRef.current) {
      const id = streamingIdRef.current
      const content = finalContent ?? streamingContentRef.current
      const updated = messagesRef.current.map(m =>
        m.id === id ? { ...m, status: 'completed' as const, content: m.content || content } : m
      )
      messagesRef.current = updated
      setMessages(updated)
      setStreamingContent('')
      streamingContentRef.current = ''
      streamingIdRef.current = null
    }
  }, [])

  const finalizeWithBlocks = useCallback((finalContent: string, blocks: ContentBlock[]) => {
    if (streamingIdRef.current) {
      const id = streamingIdRef.current
      const updated = messagesRef.current.map(m =>
        m.id === id ? { ...m, status: 'completed' as const, content: m.content || finalContent, content_blocks: blocks.length > 0 ? blocks : m.content_blocks } : m
      )
      messagesRef.current = updated
      setMessages(updated)
      setStreamingContent('')
      streamingContentRef.current = ''
      streamingIdRef.current = null
    }
  }, [])

  const updateContentBlocks = useCallback((blocks: ContentBlock[]) => {
    let targetId = streamingIdRef.current
    if (!targetId) {
      for (let i = messagesRef.current.length - 1; i >= 0; i--) {
        if (messagesRef.current[i].role === 'assistant' && messagesRef.current[i].status === 'streaming') {
          targetId = messagesRef.current[i].id
          break
        }
      }
    }
    if (!targetId) {
      // 尚无流式消息（如首轮直接调用子智能体、无文本输出）：有内容块时创建容器消息
      if (blocks.length === 0) return
      targetId = generateMessageId()
      const newMessage: Message = {
        id: targetId,
        role: 'assistant',
        content: '',
        timestamp: Date.now(),
        status: 'streaming',
      }
      messagesRef.current = [...messagesRef.current, newMessage]
      streamingIdRef.current = targetId
    }
    const updated = messagesRef.current.map(m =>
      m.id === targetId ? { ...m, content_blocks: blocks } : m
    )
    messagesRef.current = updated
    setMessages(updated)
  }, [])

  const startNewConversation = useCallback(() => {
    messagesRef.current = []
    setMessages([])
    setStreamingContent('')
    streamingContentRef.current = ''
    streamingIdRef.current = null
    setSessionId(generateSessionId())
  }, [])

  const loadConversation = useCallback((sid: string, msgs: Message[]) => {
    messagesRef.current = msgs
    setMessages(msgs)
    setStreamingContent('')
    streamingContentRef.current = ''
    streamingIdRef.current = null
    setSessionId(sid)
  }, [])

  return { messages, sessionId, streamingContent, sendMessage, addAssistantMessage, appendStreamChunk, finalizeStream, finalizeWithBlocks, updateContentBlocks, startNewConversation, loadConversation }
}
