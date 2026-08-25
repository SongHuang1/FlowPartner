import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { UserMessage } from './UserMessage'
import type { Message } from '@/types'

function makeMessage(content: string): Message {
  return { id: 'm1', role: 'user', content, timestamp: 1 }
}

describe('UserMessage', () => {
  it('renders message content', () => {
    render(<UserMessage message={makeMessage('普通消息')} />)
    expect(screen.getByText(/普通消息/)).toBeInTheDocument()
  })

  it('highlights known agent mentions', () => {
    render(<UserMessage message={makeMessage('@翻译官 帮我翻译')} agentNames={new Set(['翻译官'])} />)
    const mention = screen.getByTestId('agent-mention')
    expect(mention.textContent).toBe('@翻译官')
    expect(screen.getByText(/帮我翻译/)).toBeInTheDocument()
  })

  it('does not highlight unknown @words', () => {
    render(<UserMessage message={makeMessage('@路人 你好')} agentNames={new Set(['翻译官'])} />)
    expect(screen.queryByTestId('agent-mention')).toBeNull()
    expect(screen.getByText(/你好/)).toBeInTheDocument()
  })

  it('renders plain content when no agentNames provided', () => {
    render(<UserMessage message={makeMessage('@翻译官 你好')} />)
    expect(screen.queryByTestId('agent-mention')).toBeNull()
    expect(screen.getByText(/你好/)).toBeInTheDocument()
  })
})
