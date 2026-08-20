import { describe, it, expect } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { IterationStepView } from './IterationStepView'
import type { IterationStep } from '@/types'

function step(overrides: Partial<IterationStep> = {}): IterationStep {
  return {
    iteration: 1,
    thinking: '',
    toolCalls: [],
    ...overrides,
  }
}

describe('IterationStepView', () => {
  it('renders step number', () => {
    render(<IterationStepView step={step()} isLast={true} />)
    expect(screen.getByText('第 1 轮')).toBeInTheDocument()
  })

  it('shows friendly banner when bash deletion is blocked', () => {
    const s = step({
      toolCalls: [
        {
          tool: 'bash',
          args: { command: 'rm old.log' },
          call_id: 'c1',
          result: '{"success":false,"result":"删除操作已被拦截...","error_code":"TOOL_DELETION_BLOCKED"}',
        },
      ],
    })
    render(<IterationStepView step={s} isLast={true} />)
    // 展开后显示拦截横幅
    expect(screen.getByText(/已拦截删除命令/)).toBeInTheDocument()
    expect(screen.getByText('已拦截')).toBeInTheDocument()
  })

  it('shows friendly banner when trash not configured', () => {
    const s = step({
      toolCalls: [
        {
          tool: 'trash',
          args: { path: 'old.log' },
          call_id: 'c2',
          result: '{"success":false,"result":"回收站目录未配置...","error_code":"TOOL_TRASH_NOT_CONFIGURED"}',
        },
      ],
    })
    render(<IterationStepView step={s} isLast={true} />)
    expect(screen.getByText(/回收站未配置/)).toBeInTheDocument()
  })

  it('does not show banner for normal tool results', () => {
    const s = step({
      toolCalls: [
        {
          tool: 'read',
          args: { path: 'a.txt' },
          call_id: 'c3',
          result: 'file content',
        },
      ],
    })
    render(<IterationStepView step={s} isLast={true} />)
    expect(screen.queryByText(/已拦截/)).not.toBeInTheDocument()
    expect(screen.queryByText(/回收站未配置/)).not.toBeInTheDocument()
  })

  it('toggles expansion on click', () => {
    const s = step({
      toolCalls: [{ tool: 'bash', args: { command: 'ls' }, call_id: 'c4', result: 'a.txt' }],
    })
    render(<IterationStepView step={s} isLast={false} />)
    // 默认折叠，结果不可见
    expect(screen.queryByText('a.txt')).not.toBeInTheDocument()
    // 展开步骤头
    fireEvent.click(screen.getByRole('button'))
    // 工具调用块自身默认折叠，结果仍不可见
    expect(screen.queryByText('a.txt')).not.toBeInTheDocument()
    // 展开工具调用块
    fireEvent.click(screen.getByRole('button', { name: /bash工具调用/ }))
    expect(screen.getByText('a.txt')).toBeInTheDocument()
  })
})