import { describe, it, expect } from 'vitest'
import { buildHistoryContentBlocks, buildToolResultMap } from './history'
import type { HistoryMessage, SubAgentRun } from '@/types'

const run: SubAgentRun = {
  agent_id: 'a1',
  agent_name: '测试A',
  depth: 1,
  span_id: 'span-1',
  trace_id: 't1',
  status: 'done',
  task: '任务A',
  result: '你好',
  steps: [{ step_type: 'final_answer', content: '你好' }],
}

describe('buildToolResultMap', () => {
  it('maps tool_call_id to result content', () => {
    const messages: HistoryMessage[] = [
      { role: 'assistant', content: '', tool_calls: [{ id: 'c1', type: 'function', function: { name: 'agent__a', arguments: '{}' } }] },
      { role: 'tool', tool_call_id: 'c1', content: '结果文本' },
    ]
    const map = buildToolResultMap(messages)
    expect(map.get('c1')).toBe('结果文本')
  })
})

describe('buildHistoryContentBlocks', () => {
  it('rebuilds full card from refs + subagents detail file', () => {
    const m: HistoryMessage = {
      role: 'assistant',
      content: '我说完了',
      tool_calls: [{ id: 'c1', type: 'function', function: { name: 'agent__a1', arguments: '{"task":"任务A"}' } }],
      subagent_refs: [{ call_id: 'c1', span_id: 'span-1' }],
    }
    const blocks = buildHistoryContentBlocks(m, { 'span-1': run }, new Map())

    expect(blocks).toHaveLength(2)
    expect(blocks[0]).toEqual({ type: 'text', content: '我说完了' })
    expect(blocks[1]).toMatchObject({ type: 'subagent', agent_name: '测试A', status: 'done', result: '你好' })
  })

  it('falls back to tool_calls + tool results when no refs exist (legacy format)', () => {
    const m: HistoryMessage = {
      role: 'assistant',
      content: '',
      tool_calls: [{
        id: 'call_0aa10bb4',
        type: 'function',
        function: { name: 'agent__8ee26567', arguments: '{"task": "请说\\"你好\\""}' },
      }],
    }
    const blocks = buildHistoryContentBlocks(m, {}, new Map([['call_0aa10bb4', '你好']]))

    expect(blocks).toHaveLength(1)
    expect(blocks[0]).toEqual({
      type: 'subagent',
      span_id: 'call_0aa10bb4',
      agent_name: '子智能体',
      task: '请说"你好"',
      status: 'done',
      steps: [],
      result: '你好',
    })
  })

  it('falls back to subagent_results for oldest format', () => {
    const m: HistoryMessage = {
      role: 'assistant',
      content: '完成',
      subagent_results: [{ span_id: 's9', agent_name: '旧智能体', task: '旧任务', content: '旧结果', status: 'done' }],
    }
    const blocks = buildHistoryContentBlocks(m, {}, new Map())

    expect(blocks).toHaveLength(2)
    expect(blocks[1]).toMatchObject({ type: 'subagent', agent_name: '旧智能体', result: '旧结果', status: 'done' })
  })

  it('marks error cards from run error field', () => {
    const errRun: SubAgentRun = { ...run, span_id: 'span-e', status: 'error', error: '调用失败', result: '' }
    const m: HistoryMessage = {
      role: 'assistant',
      content: '',
      tool_calls: [{ id: 'c1', type: 'function', function: { name: 'agent__a1', arguments: '{}' } }],
      subagent_refs: [{ call_id: 'c1', span_id: 'span-e' }],
    }
    const blocks = buildHistoryContentBlocks(m, { 'span-e': errRun }, new Map())
    expect(blocks[0]).toMatchObject({ type: 'subagent', status: 'error', error: '调用失败' })
  })

  it('returns text-only blocks for plain assistant message', () => {
    const m: HistoryMessage = { role: 'assistant', content: '纯文本回复' }
    const blocks = buildHistoryContentBlocks(m, {}, new Map())
    expect(blocks).toEqual([{ type: 'text', content: '纯文本回复' }])
  })
})
