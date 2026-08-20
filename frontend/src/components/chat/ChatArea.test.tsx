import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ChatArea } from './ChatArea'
import type { UseConversationReturn } from '@/hooks/useConversation'

const mockSendMessage = vi.fn()
const mockUpdateSettings = vi.fn()
const mockWsSendMessage = vi.fn()
const mockSendCancel = vi.fn()
const mockSendPermissionResponse = vi.fn()
const mockOnFinalAnswer = vi.fn(() => () => {})
const mockOnError = vi.fn(() => () => {})
const mockOnSecurityEvent = vi.fn(() => () => {})
const mockOnPermissionRequest = vi.fn(() => () => {})

const mockLockStatus = {
  locked: false,
  failed_attempts: 0,
  has_api_key: true,
}

function makeConversation(overrides: Partial<UseConversationReturn> = {}): UseConversationReturn {
  return {
    messages: [],
    sessionId: 'sess_test_1',
    sendMessage: mockSendMessage,
    addAssistantMessage: vi.fn(),
    addToolMessage: vi.fn(),
    addAssistantToolCalls: vi.fn(),
    startNewConversation: vi.fn(),
    loadConversation: vi.fn(),
    ...overrides,
  }
}

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({
    settings: {
      model: 'gpt-4',
      agent_id: 'default',
      context_window: 8192,
      working_directory: '/test/path',
      language: 'zh-CN',
    },
    loading: false,
    error: null,
    updateSettings: mockUpdateSettings,
  }),
}))

vi.mock('@/hooks/useLock', () => ({
  useLock: () => ({
    lockStatus: mockLockStatus,
    loading: false,
    error: null,
    unlock: vi.fn(),
    lock: vi.fn(),
    refreshStatus: vi.fn(),
  }),
}))

vi.mock('@/hooks/useWebSocket', () => ({
  useWebSocket: () => ({
    connected: true,
    reconnecting: false,
    reconnectAttempts: 0,
    isReconnectExhausted: false,
    processing: false,
    sendMessage: mockWsSendMessage,
    sendCancel: mockSendCancel,
    sendPermissionResponse: mockSendPermissionResponse,
    events: [],
    manualReconnect: vi.fn(),
    onFinalAnswer: mockOnFinalAnswer,
    onError: mockOnError,
    onSecurityEvent: mockOnSecurityEvent,
    onPermissionRequest: mockOnPermissionRequest,
  }),
}))

describe('ChatArea empty state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockLockStatus.locked = false
    mockWsSendMessage.mockReturnValue(true)
    mockSendMessage.mockReturnValue([])
  })

  it('renders welcome message when no messages', () => {
    render(<ChatArea conversation={makeConversation()} />)
    expect(screen.getByText('你好！我是 FlowPartner')).toBeInTheDocument()
  })

  it('renders input in empty state', () => {
    render(<ChatArea conversation={makeConversation()} />)
    expect(screen.getByPlaceholderText('输入消息...')).toBeInTheDocument()
  })

  it('renders bottom info bar with settings', () => {
    render(<ChatArea conversation={makeConversation()} />)
    expect(screen.getByText(/模型: gpt-4/)).toBeInTheDocument()
    expect(screen.getByText(/智能体: default/)).toBeInTheDocument()
    expect(screen.getByText(/上下文: 8192/)).toBeInTheDocument()
  })

  it('renders working directory when set', () => {
    render(<ChatArea conversation={makeConversation()} />)
    expect(screen.getByText(/路径: \/test\/path/)).toBeInTheDocument()
  })

  it('send button is disabled when input is empty', () => {
    render(<ChatArea conversation={makeConversation()} />)
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })

  it('sends message when clicking send button with input', () => {
    render(<ChatArea conversation={makeConversation()} />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'hello' } })
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeEnabled()
    fireEvent.click(button)
    expect(mockSendMessage).toHaveBeenCalledWith('hello')
    expect(mockWsSendMessage).toHaveBeenCalledWith('hello', 'sess_test_1', [])
  })

  it('disables input and send button when API key is locked', () => {
    mockLockStatus.locked = true
    render(<ChatArea conversation={makeConversation()} />)
    const input = screen.getByPlaceholderText('输入消息...') as HTMLInputElement
    expect(input.disabled).toBe(true)
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })
})