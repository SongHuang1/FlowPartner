import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { ChatArea } from './ChatArea'
import type { UseConversationReturn } from '@/hooks/useConversation'
import type { SubAgentRun } from '@/types'

const mockSendMessage = vi.fn()
const mockUpdateSettings = vi.fn()
const mockWsSendMessage = vi.fn()
const mockSendCancel = vi.fn()
const mockSendPermissionResponse = vi.fn()
const mockOnStreamChunk = vi.fn(() => () => {})
const mockOnFinalAnswer = vi.fn(() => () => {})
const mockOnError = vi.fn(() => () => {})
const mockOnSecurityEvent = vi.fn(() => () => {})
const mockOnPermissionRequest = vi.fn(() => () => {})
const mockListAgents = vi.fn()

vi.mock('@/lib/api', () => ({
  listAgents: () => mockListAgents(),
}))

const mockLockStatus = {
  locked: false,
  failed_attempts: 0,
  has_api_key: true,
}

function makeConversation(overrides: Partial<UseConversationReturn> = {}): UseConversationReturn {
  return {
    messages: [],
    sessionId: 'sess_test_1',
    streamingContent: '',
    subagentResults: [],
    sendMessage: mockSendMessage,
    addAssistantMessage: vi.fn(),
    appendStreamChunk: vi.fn(),
    finalizeStream: vi.fn(),
    addSubAgentStart: vi.fn(),
    appendSubAgentChunk: vi.fn(),
    finalizeSubAgent: vi.fn(),
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
  useWebSocket: () => mockUseWebSocket(),
}))

const mockUseWebSocket = vi.fn()

const baseWsReturn = {
  connected: true,
  reconnecting: false,
  reconnectAttempts: 0,
  isReconnectExhausted: false,
  processing: false,
  sendMessage: mockWsSendMessage,
  sendCancel: mockSendCancel,
  sendPermissionResponse: mockSendPermissionResponse,
  events: [],
  steps: [],
  subagentRuns: [],
  manualReconnect: vi.fn(),
  onStreamChunk: mockOnStreamChunk,
  onFinalAnswer: mockOnFinalAnswer,
  onError: mockOnError,
  onSecurityEvent: mockOnSecurityEvent,
  onPermissionRequest: mockOnPermissionRequest,
}

const mockAgents = [
  { id: 'main', name: '主智能体', description: '默认执行者' },
  { id: 'agent-1', name: '翻译官', description: '翻译' },
]

describe('ChatArea empty state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockLockStatus.locked = false
    mockWsSendMessage.mockReturnValue(true)
    mockSendMessage.mockReturnValue([])
    mockListAgents.mockResolvedValue(mockAgents)
    mockUseWebSocket.mockReturnValue(baseWsReturn)
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

  it('sends message when clicking send button with input', async () => {
    render(<ChatArea conversation={makeConversation()} />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'hello' } })
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeEnabled()
    fireEvent.click(button)
    expect(mockSendMessage).toHaveBeenCalledWith('hello')
    await waitFor(() => {
      expect(mockWsSendMessage).toHaveBeenCalledWith('hello', 'sess_test_1', [], 'main', undefined)
    })
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

describe('ChatArea multi-agent', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockLockStatus.locked = false
    mockWsSendMessage.mockReturnValue(true)
    mockSendMessage.mockReturnValue([])
    mockListAgents.mockResolvedValue(mockAgents)
    mockUseWebSocket.mockReturnValue(baseWsReturn)
  })

  function renderWithMessages() {
    render(
      <ChatArea
        conversation={makeConversation({
          messages: [
            { id: 'm1', role: 'user', content: '你好', timestamp: 1 },
            { id: 'm2', role: 'assistant', content: '好的', timestamp: 1 },
          ],
        })}
      />
    )
  }

  it('strips @mention and passes inject_agent_id', async () => {
    renderWithMessages()
    await screen.findByRole('option', { name: '主智能体' })
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '帮我 @翻译官 翻译这句话' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(mockSendMessage).toHaveBeenCalledWith('帮我 翻译这句话')
    await waitFor(() => {
      expect(mockWsSendMessage).toHaveBeenCalledWith('帮我 翻译这句话', 'sess_test_1', [], 'main', 'agent-1')
    })
  })

  it('rejects @mention targeting the executor itself', async () => {
    renderWithMessages()
    await screen.findByRole('option', { name: '主智能体' })
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@主智能体 帮我' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(mockWsSendMessage).not.toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText(/不能强制调用会话执行者本身/)).toBeInTheDocument()
    })
  })

  it('rejects empty message after stripping mention', async () => {
    renderWithMessages()
    await screen.findByRole('option', { name: '主智能体' })
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@翻译官' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(mockWsSendMessage).not.toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText(/消息内容为空/)).toBeInTheDocument()
    })
  })

  it('passes selected executor agent id', async () => {
    renderWithMessages()
    await screen.findByRole('option', { name: '翻译官' })
    fireEvent.change(screen.getByLabelText('会话执行者'), { target: { value: 'agent-1' } })
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'hello' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    await waitFor(() => {
      expect(mockWsSendMessage).toHaveBeenCalledWith('hello', 'sess_test_1', [], 'agent-1', undefined)
    })
  })
})

describe('ChatArea subagent cards', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockLockStatus.locked = false
    mockWsSendMessage.mockReturnValue(true)
    mockSendMessage.mockReturnValue([])
    mockListAgents.mockResolvedValue(mockAgents)
    mockUseWebSocket.mockReturnValue(baseWsReturn)
  })

  const run: SubAgentRun = {
    agent_id: 'agent-1',
    agent_name: '翻译官',
    depth: 2,
    span_id: 'span-1',
    trace_id: 'trace-1',
    parent_span_id: 'span-root',
    status: 'done',
    task: '翻译这句话',
    result: '译文',
    steps: [
      { step_type: 'thinking', content: '先理解原文' },
      { step_type: 'tool_call', tool: 'read_file', args: { path: 'a.txt' } },
      { step_type: 'final_answer', content: '译文' },
    ],
  }

  it('renders subagent card with status and opens drilldown on click', () => {
    mockUseWebSocket.mockReturnValue({ ...baseWsReturn, subagentRuns: [run] })
    const conversation = makeConversation({
      messages: [
        { id: 'm1', role: 'user', content: '你好', timestamp: 1 },
        { id: 'm2', role: 'assistant', content: '好的', timestamp: 1 },
      ],
    })
    render(<ChatArea conversation={conversation} />)

    expect(screen.getByText('子智能体任务')).toBeInTheDocument()
    expect(screen.getByText('翻译官')).toBeInTheDocument()
    expect(screen.getByText(/层级 2/)).toBeInTheDocument()
    expect(screen.getByText('已完成')).toBeInTheDocument()

    fireEvent.click(screen.getByLabelText('查看子智能体 翻译官 的执行过程'))
    expect(screen.getByRole('dialog')).toBeInTheDocument()
    expect(screen.getByText('先理解原文')).toBeInTheDocument()
    expect(screen.getByText('read_file')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '返回主会话' }))
    expect(screen.queryByRole('dialog')).not.toBeInTheDocument()
  })
})