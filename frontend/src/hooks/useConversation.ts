import { useState, useRef, useCallback } from 'react'
import type { Message, ToolCall } from '@/types'

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
  addToolMessage: (toolCallId: string, name: string, content: string) => void
  addAssistantToolCalls: (toolCalls: ToolCall[]) => void
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

  const addAssistantToolCalls = useCallback((toolCalls: ToolCall[]) => {
    const lastMsg = messagesRef.current[messagesRef.current.length - 1]
    if (lastMsg && lastMsg.role === 'assistant' && lastMsg.status === 'streaming') {
      const updated = [...messagesRef.current]
      updated[updated.length - 1] = { ...lastMsg, tool_calls: toolCalls }
      messagesRef.current = updated
      setMessages(updated)
    }
  }, [])

  const addToolMessage = useCallback((toolCallId: string, name: string, _content: string) => {
    const newMessage: Message = {
      id: generateMessageId(),
      role: 'assistant',
      content: '',
      timestamp: Date.now(),
      tool_call_id: toolCallId,
      name,
      status: 'completed',
    }
    const updated = [...messagesRef.current, newMessage]
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

  return { messages, sessionId, streamingContent, sendMessage, addAssistantMessage, appendStreamChunk, finalizeStream, addToolMessage, addAssistantToolCalls, startNewConversation, loadConversation }
}
