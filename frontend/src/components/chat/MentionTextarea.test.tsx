import { describe, it, expect, vi } from 'vitest'
import { useState } from 'react'
import { render, screen, fireEvent } from '@testing-library/react'
import { MentionTextarea } from './MentionTextarea'
import { detectMention } from '@/lib/mention'
import type { AgentMeta } from '@/types'

const agents: AgentMeta[] = [
  { id: 'a1', name: '翻译官', description: '翻译文本' },
  { id: 'a2', name: '测试员', description: '执行测试' },
]

function Wrapper({ initial = '', onSend }: { initial?: string; onSend?: (e: React.KeyboardEvent) => void }) {
  const [value, setValue] = useState(initial)
  return (
    <MentionTextarea
      value={value}
      onChange={setValue}
      onKeyDown={onSend}
      agents={agents}
      placeholder="输入消息..."
    />
  )
}

describe('detectMention', () => {
  it('detects @ at start of text', () => {
    expect(detectMention('@翻', 2)).toEqual({ start: 0, query: '翻' })
  })

  it('detects @ after whitespace', () => {
    expect(detectMention('你好 @测', 5)).toEqual({ start: 3, query: '测' })
  })

  it('returns null when @ is preceded by non-whitespace', () => {
    expect(detectMention('abc@x', 5)).toBeNull()
  })

  it('returns null when cursor word contains whitespace before @', () => {
    expect(detectMention('@ab x', 5)).toBeNull()
  })

  it('returns null for empty query without @', () => {
    expect(detectMention('hello', 5)).toBeNull()
  })
})

describe('MentionTextarea', () => {
  it('renders textarea with placeholder', () => {
    render(<Wrapper />)
    expect(screen.getByPlaceholderText('输入消息...')).toBeInTheDocument()
  })

  it('opens popup when typing @ followed by query', () => {
    render(<Wrapper />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@' } })
    fireEvent.keyUp(input)

    const popup = screen.getByTestId('mention-popup')
    expect(popup).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /翻译官/ })).toBeInTheDocument()
    expect(screen.getByRole('option', { name: /测试员/ })).toBeInTheDocument()
  })

  it('filters matches by query', () => {
    render(<Wrapper />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@翻' } })
    fireEvent.keyUp(input)

    expect(screen.getByRole('option', { name: /翻译官/ })).toBeInTheDocument()
    expect(screen.queryByRole('option', { name: /测试员/ })).not.toBeInTheDocument()
  })

  it('closes popup when no agent matches', () => {
    render(<Wrapper />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@不存在' } })
    fireEvent.keyUp(input)

    expect(screen.queryByTestId('mention-popup')).not.toBeInTheDocument()
  })

  it('does not trigger popup when @ is preceded by non-whitespace', () => {
    render(<Wrapper initial="abc" />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'abc@' } })
    fireEvent.keyUp(input)

    expect(screen.queryByTestId('mention-popup')).not.toBeInTheDocument()
  })

  it('inserts selected agent with trailing space and closes popup', () => {
    const { container } = render(<Wrapper />)
    const input = screen.getByPlaceholderText('输入消息...')
    // 模拟输入 "@翻"，光标在末尾
    fireEvent.change(input, { target: { value: '@翻' } })
    ;(input as HTMLTextAreaElement).setSelectionRange(2, 2)
    fireEvent.keyUp(input)
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(input).toHaveValue('@翻译官 ')
    expect(screen.queryByTestId('mention-popup')).not.toBeInTheDocument()
    expect(container).toBeDefined()
  })

  it('Enter selects mention instead of triggering external key handler', () => {
    const onExternalKeyDown = vi.fn((e: React.KeyboardEvent) => {
      if ((e as React.KeyboardEvent).key === 'Enter') throw new Error('should not send')
    })
    render(<Wrapper onSend={onExternalKeyDown} />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@' } })
    fireEvent.keyUp(input)
    expect(() => fireEvent.keyDown(input, { key: 'Enter' })).not.toThrow()
    expect(input).toHaveValue('@翻译官 ')
  })

  it('Escape closes popup and forwards subsequent Enter to external handler', () => {
    const onSend = vi.fn((e: React.KeyboardEvent) => {
      if (e.key === 'Enter') e.preventDefault()
    })
    render(<Wrapper onSend={onSend} />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@' } })
    fireEvent.keyUp(input)
    fireEvent.keyDown(input, { key: 'Escape' })
    expect(screen.queryByTestId('mention-popup')).not.toBeInTheDocument()

    fireEvent.keyDown(input, { key: 'Enter' })
    expect(onSend).toHaveBeenCalled()
  })

  it('keyboard arrows move highlight within popup', () => {
    render(<Wrapper />)
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '@' } })
    fireEvent.keyUp(input)

    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(input, { key: 'ArrowDown' })
    expect(screen.getAllByRole('option')[1]).toHaveAttribute('aria-selected', 'true')
    fireEvent.keyDown(input, { key: 'ArrowUp' })
    expect(screen.getAllByRole('option')[0]).toHaveAttribute('aria-selected', 'true')
  })

  it('highlights known agent names in mirror layer', () => {
    render(<Wrapper initial="@翻译官 你好" />)
    const overlay = document.querySelector('[aria-hidden]')
    expect(overlay).not.toBeNull()
    const highlightSpan = overlay!.querySelector('.font-semibold')
    expect(highlightSpan).not.toBeNull()
    expect(highlightSpan!.textContent).toBe('@翻译官')
    expect(overlay!.textContent).toContain('你好')
  })

  it('does not highlight unknown @words in mirror layer', () => {
    render(<Wrapper initial="@路人甲 你好" />)
    const overlay = document.querySelector('[aria-hidden]')!
    expect(overlay.querySelector('.font-semibold')).toBeNull()
  })
})
