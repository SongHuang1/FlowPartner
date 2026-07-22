import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MessageBubble } from './MessageBubble'
import type { Message } from '@/types'

function createMessage(overrides: Partial<Message> = {}): Message {
  return {
    id: 'msg_test',
    role: 'user',
    content: 'Hello',
    timestamp: 1700000000000,
    ...overrides,
  }
}

describe('MessageBubble', () => {
  it('renders user message with right alignment', () => {
    const msg = createMessage({ role: 'user', content: 'User says' })
    render(<MessageBubble message={msg} />)

    const bubble = screen.getByText('User says')
    expect(bubble).toBeInTheDocument()
    expect(bubble.parentElement?.parentElement).toHaveClass('justify-end')
  })

  it('renders assistant message with left alignment', () => {
    const msg = createMessage({ role: 'assistant', content: 'AI says' })
    render(<MessageBubble message={msg} />)

    const bubble = screen.getByText('AI says')
    expect(bubble).toBeInTheDocument()
    expect(bubble.parentElement?.parentElement).toHaveClass('justify-start')
  })

  it('applies blue background to user messages', () => {
    const msg = createMessage({ role: 'user', content: 'Blue msg' })
    render(<MessageBubble message={msg} />)

    const bubble = screen.getByText('Blue msg')
    expect(bubble).toHaveClass('bg-blue-500')
    expect(bubble).toHaveClass('text-white')
  })

  it('applies neutral gray background to assistant messages', () => {
    const msg = createMessage({ role: 'assistant', content: 'Gray msg' })
    render(<MessageBubble message={msg} />)

    const bubble = screen.getByText('Gray msg')
    expect(bubble).toHaveClass('bg-neutral-100')
    expect(bubble).toHaveClass('text-neutral-800')
  })

  it('shows FlowPartner name for assistant messages', () => {
    const msg = createMessage({ role: 'assistant', content: 'AI response' })
    render(<MessageBubble message={msg} />)

    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
  })

  it('does not show name for user messages', () => {
    const msg = createMessage({ role: 'user', content: 'User message' })
    render(<MessageBubble message={msg} />)

    expect(screen.queryByText('FlowPartner')).not.toBeInTheDocument()
  })

  it('renders message content correctly', () => {
    const content = 'This is a test message with special chars: <>&"\''
    const msg = createMessage({ content })
    render(<MessageBubble message={msg} />)

    expect(screen.getByText(content)).toBeInTheDocument()
  })

  it('renders long content without truncation', () => {
    const longContent = 'A'.repeat(500)
    const msg = createMessage({ content: longContent })
    render(<MessageBubble message={msg} />)

    expect(screen.getByText(longContent)).toBeInTheDocument()
  })

  it('renders empty content', () => {
    const msg = createMessage({ content: '' })
    const { container } = render(<MessageBubble message={msg} />)

    // Should still render the bubble structure
    const bubble = container.querySelector('.rounded-lg')
    expect(bubble).toBeInTheDocument()
  })

  it('has max-width constraint on the wrapper', () => {
    const msg = createMessage({ content: 'Test' })
    const { container } = render(<MessageBubble message={msg} />)

    const wrapper = container.querySelector('.max-w-\\[75\\%\\]')
    expect(wrapper).toBeInTheDocument()
  })

  it('renders unicode content correctly', () => {
    const unicodeContent = '你好世界 🌍 مرحبا'
    const msg = createMessage({ content: unicodeContent })
    render(<MessageBubble message={msg} />)

    expect(screen.getByText(unicodeContent)).toBeInTheDocument()
  })

  it('renders multiline content', () => {
    const multilineContent = 'Line 1\nLine 2\nLine 3'
    const msg = createMessage({ content: multilineContent })
    render(<MessageBubble message={msg} />)

    expect(screen.getByText(/Line 1/)).toBeInTheDocument()
  })
})
