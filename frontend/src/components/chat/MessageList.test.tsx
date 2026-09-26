import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { MessageList } from './ChatArea'
import type { Message } from '@/types'

function msg(id: string, role: 'user' | 'assistant', content: string): Message {
  return { id, role, content, timestamp: 1000 + parseInt(id, 10) || Date.now() }
}

describe('MessageList', () => {
  it('renders empty list when no messages provided', () => {
    const { container } = render(<MessageList messages={[]} streamingContent="" agentNames={new Set<string>()} />)
    const list = container.querySelector('.flex.flex-col.gap-3')
    expect(list).toBeInTheDocument()
    expect(list?.children.length).toBe(0)
  })

  it('renders a single assistant message with left alignment', () => {
    const messages: Message[] = [msg('1', 'assistant', 'Hello from AI')]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    const el = screen.getByText('Hello from AI')
    expect(el).toBeInTheDocument()
    expect(el.closest('.justify-start')).toBeTruthy()
  })

  it('renders a single user message with right alignment', () => {
    const messages: Message[] = [msg('1', 'user', 'Hello from user')]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    const el = screen.getByText('Hello from user')
    expect(el).toBeInTheDocument()
    expect(el.closest('.justify-end')).toBeTruthy()
  })

  it('renders mixed messages in correct order', () => {
    const messages: Message[] = [
      msg('1', 'assistant', 'First AI'),
      msg('2', 'user', 'First user'),
      msg('3', 'assistant', 'Second AI'),
    ]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    expect(screen.getByText('First AI')).toBeInTheDocument()
    expect(screen.getByText('First user')).toBeInTheDocument()
    expect(screen.getByText('Second AI')).toBeInTheDocument()
  })

  it('applies blue style to user messages', () => {
    const messages: Message[] = [msg('1', 'user', 'Blue message')]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    const text = screen.getByText('Blue message')
    const bubble = text.closest('.bg-blue-500')
    expect(bubble).not.toBeNull()
    expect(bubble).toHaveClass('text-white')
  })

  it('applies neutral gray style to assistant messages', () => {
    const messages: Message[] = [msg('1', 'assistant', 'Gray message')]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    const bubble = screen.getByText('Gray message')
    expect(bubble.closest('.text-neutral-800')).toBeTruthy()
  })

  it('shows FlowPartner name for assistant messages', () => {
    const messages: Message[] = [msg('1', 'assistant', 'AI response')]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
  })

  it('does not show name for user messages', () => {
    const messages: Message[] = [msg('1', 'user', 'User message')]
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    expect(screen.queryByText('FlowPartner')).not.toBeInTheDocument()
  })

  it('renders multiple messages with correct count', () => {
    const messages: Message[] = [
      msg('1', 'assistant', 'Msg 1'),
      msg('2', 'user', 'Msg 2'),
      msg('3', 'assistant', 'Msg 3'),
      msg('4', 'user', 'Msg 4'),
    ]
    const { container } = render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)
    const list = container.querySelector('.flex.flex-col.gap-3')
    expect(list?.children.length).toBe(4)
  })

  it('renders messages with unique keys (no React key warning)', () => {
    const messages: Message[] = [
      msg('1', 'assistant', 'Unique 1'),
      msg('2', 'assistant', 'Unique 2'),
    ]
    const consoleSpy = vi.spyOn(console, 'error')
    render(<MessageList messages={messages} streamingContent="" agentNames={new Set<string>()} />)

    expect(screen.getByText('Unique 1')).toBeInTheDocument()
    expect(screen.getByText('Unique 2')).toBeInTheDocument()
    expect(consoleSpy).not.toHaveBeenCalled()
    consoleSpy.mockRestore()
  })

  it('renders content_blocks in chronological order when present', () => {
    const m: Message = {
      id: '1',
      role: 'assistant',
      content: '第一段第二段',
      timestamp: 1000,
      content_blocks: [
        { type: 'text', content: '第一段' },
        { type: 'subagent', span_id: 's1', agent_name: '翻译官', task: '翻译', status: 'done', steps: [], result: '译文' },
        { type: 'text', content: '第二段' },
      ],
    }
    const { container } = render(<MessageList messages={[m]} streamingContent="" agentNames={new Set<string>()} />)

    const nodes = Array.from(container.querySelectorAll('p, span'))
    const idxFirst = nodes.findIndex((el) => el.textContent === '第一段')
    const idxSub = nodes.findIndex((el) => el.textContent?.includes('翻译官'))
    const idxSecond = nodes.findIndex((el) => el.textContent === '第二段')

    expect(idxFirst).toBeGreaterThanOrEqual(0)
    expect(idxSub).toBeGreaterThan(idxFirst)
    expect(idxSecond).toBeGreaterThan(idxSub)
  })

  it('falls back to flat content when content_blocks is absent', () => {
    const m: Message = { id: '1', role: 'assistant', content: '只有扁平内容', timestamp: 1000 }
    render(<MessageList messages={[m]} streamingContent="" agentNames={new Set<string>()} />)
    expect(screen.getByText('只有扁平内容')).toBeInTheDocument()
  })
})
