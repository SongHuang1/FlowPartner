import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SubAgentCard } from './SubAgentCard'
import type { SubAgentRun } from '@/types'

function makeRun(overrides: Partial<SubAgentRun> = {}): SubAgentRun {
  return {
    agent_id: 'agent-1',
    agent_name: '翻译官',
    depth: 2,
    span_id: 'span-1',
    trace_id: 'trace-1',
    parent_span_id: 'span-root',
    status: 'running',
    task: '翻译这句话',
    steps: [],
    ...overrides,
  }
}

describe('SubAgentCard', () => {
  it('renders agent name, depth and running status', () => {
    render(<SubAgentCard run={makeRun()} onClick={() => {}} />)
    expect(screen.getByText('翻译官')).toBeInTheDocument()
    expect(screen.getByText(/层级 2/)).toBeInTheDocument()
    expect(screen.getByText('运行中')).toBeInTheDocument()
  })

  it('shows task while running and result when done', () => {
    const { rerender } = render(<SubAgentCard run={makeRun()} onClick={() => {}} />)
    expect(screen.getByText('翻译这句话')).toBeInTheDocument()

    rerender(
      <SubAgentCard run={makeRun({ status: 'done', result: '译文' })} onClick={() => {}} />
    )
    expect(screen.getByText('已完成')).toBeInTheDocument()
    expect(screen.getByText('译文')).toBeInTheDocument()
  })

  it('shows error message when failed', () => {
    render(
      <SubAgentCard run={makeRun({ status: 'error', error: '调用失败' })} onClick={() => {}} />
    )
    expect(screen.getByText('失败')).toBeInTheDocument()
    expect(screen.getByText('调用失败')).toBeInTheDocument()
  })

  it('calls onClick with the run when clicked', () => {
    const onClick = vi.fn()
    const run = makeRun()
    render(<SubAgentCard run={run} onClick={onClick} />)
    fireEvent.click(screen.getByLabelText('查看子智能体 翻译官 的执行过程'))
    expect(onClick).toHaveBeenCalledWith(run)
  })
})
