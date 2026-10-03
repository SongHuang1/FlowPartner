import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { ChatArea } from './ChatArea'
import { useConversation } from '@/hooks/useConversation'

/** 用真实的 useConversation，让事件流真正写入消息的 content_blocks */
function Harness() {
  const conversation = useConversation()
  return <ChatArea conversation={conversation} />
}

type ThreadEventHandler = (method: string, params: unknown) => void

let onThreadEvent: ThreadEventHandler

vi.mock('@/hooks/useWebSocket', () => ({
  useWsV2: (cbs: { onThreadEvent?: ThreadEventHandler }) => {
    onThreadEvent = cbs.onThreadEvent!
    return {
      connectionState: 'connected',
      reconnectAttempts: 0,
      connect: vi.fn(),
      startThread: vi.fn(),
      startChat: vi.fn(),
      interrupt: vi.fn(),
      respondToApproval: vi.fn(),
    }
  },
}))

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({ settings: {} }),
}))

vi.mock('@/hooks/useLock', () => ({
  useLock: () => ({ lockStatus: { locked: false, failed_attempts: 0, has_api_key: true } }),
}))

vi.mock('@/lib/api', () => ({
  listAgents: () => Promise.resolve([]),
}))

/** 按后端 events.go 的真实形状构造事件 */
function started(itemId: string, type: string) {
  return { item: { itemId, type } }
}

function completed(itemId: string, type: string, payload: Record<string, unknown>) {
  // 注意：后端把 payload 放进 item.text
  return { item: { itemId, type, text: JSON.stringify(payload) } }
}

function fire(method: string, params: unknown) {
  act(() => { onThreadEvent(method, params) })
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('ChatArea tool call event handling', () => {
  it('renders a bash tool call using the payload carried on item.text', () => {
    render(<Harness />)

    fire('turn/started', { turnId: 'u1' })
    fire('item/started', started('it1', 'commandExecution'))
    fire('item/completed', completed('it1', 'commandExecution', {
      success: true, result: 'total 0', error_code: '',
      tool_name: 'bash', arguments: { command: 'ls -la' },
    }))
    fire('turn/completed', {})

    // 工具名 + 命令 + 状态都来自 item.text 里的 payload
    expect(screen.getByText('bash')).toBeInTheDocument()
    expect(screen.getByText('ls -la')).toBeInTheDocument()
    expect(screen.getByText('执行成功')).toBeInTheDocument()
  })

  it('renders write tools whose item type is patchApply', () => {
    render(<Harness />)

    fire('turn/started', { turnId: 'u1' })
    fire('item/started', started('it2', 'patchApply'))
    fire('item/completed', completed('it2', 'patchApply', {
      success: true, result: 'written', error_code: '',
      tool_name: 'write', arguments: { path: 'notes.md' },
    }))
    fire('turn/completed', {})

    expect(screen.getByText('write')).toBeInTheDocument()
    expect(screen.getByText('notes.md')).toBeInTheDocument()
  })

  it('marks execution failure separately from a successful call', () => {
    render(<Harness />)

    fire('turn/started', { turnId: 'u1' })
    fire('item/started', started('it3', 'commandExecution'))
    fire('item/completed', completed('it3', 'commandExecution', {
      success: false, result: '命令返回非零', error_code: 'TOOL_ERROR',
      tool_name: 'bash', arguments: { command: 'ls' },
    }))
    fire('turn/completed', {})

    expect(screen.getByText('执行失败')).toBeInTheDocument()
    expect(screen.getByText('命令返回非零')).toBeInTheDocument()
  })

  it('shows a running placeholder before the tool finishes', () => {
    render(<Harness />)

    fire('turn/started', { turnId: 'u1' })
    fire('item/started', started('it4', 'commandExecution'))

    expect(screen.getByText('执行中')).toBeInTheDocument()
  })

  it('does not create a tool call block for subagent invocations', () => {
    render(<Harness />)

    fire('turn/started', { turnId: 'u1' })
    fire('item/started', started('it5', 'commandExecution'))
    fire('subagent/subagent_start', {
      payload: { span_id: 'sp1', agent_name: '翻译官', task: '翻译', depth: 1 },
    })
    fire('item/completed', completed('it5', 'commandExecution', {
      success: true, result: '译文', error_code: '',
      tool_name: 'agent__x', arguments: { task: '翻译' },
    }))
    fire('subagent/subagent_end', { payload: { span_id: 'sp1', result: '译文' } })
    fire('turn/completed', {})

    // 不得出现 agent__x 的工具横条
    expect(screen.queryByText('agent__x')).not.toBeInTheDocument()
    // 子智能体本身仍正常展示
    expect(screen.getByText('翻译官')).toBeInTheDocument()
  })

  it('never surfaces the raw json envelope to the user', () => {
    render(<Harness />)

    fire('turn/started', { turnId: 'u1' })
    fire('item/started', started('it6', 'commandExecution'))
    fire('item/completed', completed('it6', 'commandExecution', {
      success: true, result: 'hello', error_code: '',
      tool_name: 'read', arguments: { path: 'a.txt' },
    }))
    fire('turn/completed', {})

    expect(screen.getByText('a.txt')).toBeInTheDocument()
    expect(screen.queryByText(/"tool_name"/)).not.toBeInTheDocument()
    expect(screen.queryByText(/"error_code"/)).not.toBeInTheDocument()
  })
})
