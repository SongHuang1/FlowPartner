import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MessageList } from './ChatArea'

interface Message {
  id: string
  role: 'system' | 'user-ephemeral'
  content: string
}

describe('MessageList', () => {
  it('renders empty list when no messages provided', () => {
    const { container } = render(<MessageList messages={[]} />)
    const list = container.querySelector('.flex.flex-col.gap-3')
    expect(list).toBeInTheDocument()
    expect(list?.children.length).toBe(0)
  })

  it('renders a single system message with left alignment', () => {
    const messages: Message[] = [
      { id: '1', role: 'system', content: 'Hello from system' },
    ]
    render(<MessageList messages={messages} />)

    const msg = screen.getByText('Hello from system')
    expect(msg).toBeInTheDocument()
    // System messages should be in a justify-start container
    expect(msg.parentElement).toHaveClass('justify-start')
  })

  it('renders a single user-ephemeral message with right alignment', () => {
    const messages: Message[] = [
      { id: '1', role: 'user-ephemeral', content: 'Hello from user' },
    ]
    render(<MessageList messages={messages} />)

    const msg = screen.getByText('Hello from user')
    expect(msg).toBeInTheDocument()
    // User messages should be in a justify-end container
    expect(msg.parentElement).toHaveClass('justify-end')
  })

  it('renders mixed messages in correct order', () => {
    const messages: Message[] = [
      { id: '1', role: 'system', content: 'First system' },
      { id: '2', role: 'user-ephemeral', content: 'First user' },
      { id: '3', role: 'system', content: 'Second system' },
    ]
    render(<MessageList messages={messages} />)

    expect(screen.getByText('First system')).toBeInTheDocument()
    expect(screen.getByText('First user')).toBeInTheDocument()
    expect(screen.getByText('Second system')).toBeInTheDocument()
  })

  it('applies semi-transparent blue style to user-ephemeral messages', () => {
    const messages: Message[] = [
      { id: '1', role: 'user-ephemeral', content: 'Blue message' },
    ]
    render(<MessageList messages={messages} />)

    const bubble = screen.getByText('Blue message')
    expect(bubble).toHaveClass('bg-blue-500/60')
    expect(bubble).toHaveClass('text-white')
  })

  it('applies neutral gray style to system messages', () => {
    const messages: Message[] = [
      { id: '1', role: 'system', content: 'Gray message' },
    ]
    render(<MessageList messages={messages} />)

    const bubble = screen.getByText('Gray message')
    expect(bubble).toHaveClass('bg-neutral-100')
    expect(bubble).toHaveClass('text-neutral-800')
  })

  it('renders multiple messages with correct count', () => {
    const messages: Message[] = [
      { id: '1', role: 'system', content: 'Msg 1' },
      { id: '2', role: 'user-ephemeral', content: 'Msg 2' },
      { id: '3', role: 'system', content: 'Msg 3' },
      { id: '4', role: 'user-ephemeral', content: 'Msg 4' },
    ]
    const { container } = render(<MessageList messages={messages} />)
    const list = container.querySelector('.flex.flex-col.gap-3')
    expect(list?.children.length).toBe(4)
  })

  it('renders messages with unique keys (no React key warning)', () => {
    const messages: Message[] = [
      { id: 'msg-1', role: 'system', content: 'Unique 1' },
      { id: 'msg-2', role: 'system', content: 'Unique 2' },
    ]
    // If keys are not unique, React will warn. This test verifies no crash.
    const consoleSpy = vi.spyOn(console, 'error')
    render(<MessageList messages={messages} />)

    expect(screen.getByText('Unique 1')).toBeInTheDocument()
    expect(screen.getByText('Unique 2')).toBeInTheDocument()
    expect(consoleSpy).not.toHaveBeenCalled()
    consoleSpy.mockRestore()
  })
})
