import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ChatArea } from './ChatArea'

const mockSendMessage = vi.fn()
const mockUpdateSettings = vi.fn()

vi.mock('@/hooks/useConversation', () => ({
  useConversation: () => ({
    messages: [],
    loading: false,
    error: null,
    sendMessage: mockSendMessage,
  }),
}))

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

describe('ChatArea empty state', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders welcome message when no messages', () => {
    render(<ChatArea />)
    expect(screen.getByText('Hello! I am FlowPartner')).toBeInTheDocument()
  })

  it('renders input in empty state', () => {
    render(<ChatArea />)
    expect(screen.getByPlaceholderText('Type a message...')).toBeInTheDocument()
  })

  it('renders bottom info bar with settings', () => {
    render(<ChatArea />)
    expect(screen.getByText(/model: gpt-4/)).toBeInTheDocument()
    expect(screen.getByText(/agent: default/)).toBeInTheDocument()
    expect(screen.getByText(/ctx: 8192/)).toBeInTheDocument()
  })

  it('renders working directory when set', () => {
    render(<ChatArea />)
    expect(screen.getByText(/path: \/test\/path/)).toBeInTheDocument()
  })

  it('send button is disabled when input is empty', () => {
    render(<ChatArea />)
    const button = screen.getByRole('button', { name: 'Send' })
    expect(button).toBeDisabled()
  })
})
