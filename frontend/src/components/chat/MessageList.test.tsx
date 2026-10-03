import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MessageList } from './ChatArea'
import type { Message, ContentBlock } from '@/types'

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

function subagentMsg(result: string): Message {
  return {
    id: '1',
    role: 'assistant',
    content: '',
    timestamp: 1000,
    content_blocks: [
      { type: 'subagent', span_id: 's1', agent_name: '翻译官', task: '翻译', status: 'done', steps: [], result },
    ],
  }
}

describe('AssistantMessage subagent block', () => {
  // 必须超过 tailSnippet 默认 120 字符阈值，才会触发收起态截断
  const longResult =
    '第一段内容需要足够长才能触发截断逻辑，这里继续填充以确保整体长度达标。' +
    '第二段内容同样需要很长，补充更多文字让整体长度超过既定的阈值限制。' +
    '第三段内容很长需要滚动查看，这里继续填充长度直到足够越过限制。' +
    '第四段收尾结束，后面还有一点内容确保总长度达标。'

  it('collapsed shows only the tail snippet, not the whole result', () => {
    expect(longResult.length).toBeGreaterThan(120)
    render(<MessageList messages={[subagentMsg(longResult)]} streamingContent="" agentNames={new Set<string>()} />)

    const name = screen.getByText('翻译官')
    // 名字加粗
    expect(name.className).toContain('font-semibold')

    // 收起态只展示尾部片段，不展示完整结果
    expect(screen.queryByText(longResult)).not.toBeInTheDocument()
    expect(screen.getByRole('button', { name: '展开' })).toBeInTheDocument()
  })

  it('collapsed preview area is height-capped and scrollable', () => {
    const { container } = render(
      <MessageList messages={[subagentMsg(longResult)]} streamingContent="" agentNames={new Set<string>()} />,
    )
    const preview = Array.from(container.querySelectorAll('span')).find((el) =>
      el.className.includes('max-h-[3em]'),
    )
    expect(preview).toBeDefined()
    expect(preview!.className).toContain('overflow-y-auto')
  })

  it('short result is shown in full with an inline toggle', () => {
    render(<MessageList messages={[subagentMsg('翻译完成')]} streamingContent="" agentNames={new Set<string>()} />)
    expect(screen.getByText('翻译完成')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '展开' })).toBeInTheDocument()
  })

  it('expanded shows the full result and a collapse control', () => {
    render(<MessageList messages={[subagentMsg(longResult)]} streamingContent="" agentNames={new Set<string>()} />)
    fireEvent.click(screen.getByRole('button', { name: '展开' }))

    expect(screen.getByText(longResult)).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '收起' })).toBeInTheDocument()
  })

  it('shows a running state without a toggle', () => {
    const m: Message = {
      id: '1',
      role: 'assistant',
      content: '',
      timestamp: 1000,
      content_blocks: [
        { type: 'subagent', span_id: 's1', agent_name: '翻译官', task: '翻译', status: 'running', steps: [] },
      ],
    }
    render(<MessageList messages={[m]} streamingContent="" agentNames={new Set<string>()} />)
    expect(screen.getByText('正在执行…')).toBeInTheDocument()
    expect(screen.queryByRole('button', { name: '展开' })).not.toBeInTheDocument()
  })
})

describe('AssistantMessage tool call block', () => {
  function toolMsg(over: Partial<Extract<ContentBlock, { type: 'tool_call' }>> = {}): Message {
    return {
      id: '1',
      role: 'assistant',
      content: '',
      timestamp: 1000,
      content_blocks: [
        {
          type: 'tool_call',
          call_id: 'c1',
          tool_name: 'bash',
          arguments: '{"command":"ls -la"}',
          status: 'done',
          success: true,
          summary: 'ls -la',
          result: 'total 0',
          ...over,
        },
      ],
    }
  }

  it('shows what was executed, not the raw json', () => {
    render(<MessageList messages={[toolMsg()]} streamingContent="" agentNames={new Set<string>()} />)

    expect(screen.getByText('bash')).toBeInTheDocument()
    expect(screen.getByText('ls -la')).toBeInTheDocument()
    expect(screen.getByText('执行成功')).toBeInTheDocument()
    // 原始 arguments JSON 不得直接展示
    expect(screen.queryByText(/"command":"ls -la"/)).not.toBeInTheDocument()
    expect(screen.queryByText(/"success":/)).not.toBeInTheDocument()
  })

  it('separates call failure from execution failure', () => {
    render(
      <MessageList
        messages={[toolMsg({ success: false, result: '命令返回非零' })]}
        streamingContent=""
        agentNames={new Set<string>()}
      />,
    )
    expect(screen.getByText('执行失败')).toBeInTheDocument()
    expect(screen.queryByText('调用失败')).not.toBeInTheDocument()
    expect(screen.getByText('命令返回非零')).toBeInTheDocument()
  })

  it('reports a failed call distinctly', () => {
    render(
      <MessageList
        messages={[toolMsg({ status: 'error', success: undefined, error: '连接中断' })]}
        streamingContent=""
        agentNames={new Set<string>()}
      />,
    )
    expect(screen.getByText('调用失败')).toBeInTheDocument()
    expect(screen.getByText('连接中断')).toBeInTheDocument()
  })

  it('shows a running state without success text', () => {
    render(
      <MessageList
        messages={[toolMsg({ status: 'running', success: undefined, summary: undefined, result: undefined })]}
        streamingContent=""
        agentNames={new Set<string>()}
      />,
    )
    expect(screen.getByText('执行中')).toBeInTheDocument()
    expect(screen.queryByText('执行成功')).not.toBeInTheDocument()
  })
})
