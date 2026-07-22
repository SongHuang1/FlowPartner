import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MessageList } from './ChatArea'
import type { Message } from '@/types'

function msg(id: string, role: 'user' | 'assistant', content: string): Message {
  return { id, role, content, timestamp: 1000 + parseInt(id, 10) || Date.now() }
}

describe('MessageList', () => {
  it('renders empty list when no messages provided', () => {
    const { container } = render(<MessageList messages={[]} />)
    const list = container.querySelector('.flex.flex-col.gap-3')
    expect(list).toBeInTheDocument()
    expect(list?.children.length).toBe(0)
  })

  it('renders a single assistant message with left alignment', () => {
    const messages: Message[] = [msg('1', 'assistant', 'Hello from AI')]
    render(<MessageList messages={messages} />)

    const el = screen.getByText('Hello from AI')
    expect(el).toBeInTheDocument()
    expect(el.parentElement?.parentElement).toHaveClass('justify-start')
  })

  it('renders a single user message with right alignment', () => {
    const messages: Message[] = [msg('1', 'user', 'Hello from user')]
    render(<MessageList messages={messages} />)

    const el = screen.getByText('Hello from user')
    expect(el).toBeInTheDocument()
    expect(el.parentElement?.parentElement).toHaveClass('justify-end')
  })

  it('renders mixed messages in correct order', () => {
    const messages: Message[] = [
      msg('1', 'assistant', 'First AI'),
      msg('2', 'user', 'First user'),
      msg('3', 'assistant', 'Second AI'),
    ]
    render(<MessageList messages={messages} />)

    expect(screen.getByText('First AI')).toBeInTheDocument()
    expect(screen.getByText('First user')).toBeInTheDocument()
    expect(screen.getByText('Second AI')).toBeInTheDocument()
  })

  it('applies blue style to user messages', () => {
    const messages: Message[] = [msg('1', 'user', 'Blue message')]
    render(<MessageList messages={messages} />)

    const bubble = screen.getByText('Blue message')
    expect(bubble).toHaveClass('bg-blue-500')
    expect(bubble).toHaveClass('text-white')
  })

  it('applies neutral gray style to assistant messages', () => {
    const messages: Message[] = [msg('1', 'assistant', 'Gray message')]
    render(<MessageList messages={messages} />)

    const bubble = screen.getByText('Gray message')
    expect(bubble).toHaveClass('bg-neutral-100')
    expect(bubble).toHaveClass('text-neutral-800')
  })

  it('shows FlowPartner name for assistant messages', () => {
    const messages: Message[] = [msg('1', 'assistant', 'AI response')]
    render(<MessageList messages={messages} />)

    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
  })

  it('does not show name for user messages', () => {
    const messages: Message[] = [msg('1', 'user', 'User message')]
    render(<MessageList messages={messages} />)

    expect(screen.queryByText('FlowPartner')).not.toBeInTheDocument()
  })

  it('renders multiple messages with correct count', () => {
    const messages: Message[] = [
      msg('1', 'assistant', 'Msg 1'),
      msg('2', 'user', 'Msg 2'),
      msg('3', 'assistant', 'Msg 3'),
      msg('4', 'user', 'Msg 4'),
    ]
    const { container } = render(<MessageList messages={messages} />)
    const list = container.querySelector('.flex.flex-col.gap-3')
    expect(list?.children.length).toBe(4)
  })

  it('renders messages with unique keys (no React key warning)', () => {
    const messages: Message[] = [
      msg('1', 'assistant', 'Unique 1'),
      msg('2', 'assistant', 'Unique 2'),
    ]
    const consoleSpy = vi.spyOn(console, 'error')
    render(<MessageList messages={messages} />)

    expect(screen.getByText('Unique 1')).toBeInTheDocument()
    expect(screen.getByText('Unique 2')).toBeInTheDocument()
    expect(consoleSpy).not.toHaveBeenCalled()
    consoleSpy.mockRestore()
  })
})
