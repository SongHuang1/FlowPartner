import type { ContentBlock, HistoryMessage, SubAgentRun } from '@/types'

function subagentCardFromRun(run: SubAgentRun): ContentBlock {
  return {
    type: 'subagent',
    span_id: run.span_id,
    agent_name: run.agent_name,
    task: run.task || '',
    status: run.error ? 'error' : 'done',
    steps: run.steps || [],
    result: run.error ? undefined : run.result,
    error: run.error || undefined,
  }
}

// 从历史消息构建内容块：文本 + 子智能体卡片。
// 卡片来源优先级：subagent_refs→详情文件 > tool_calls(agent__)+tool结果（旧格式兜底）> subagent_results（更早格式）
export function buildHistoryContentBlocks(
  m: HistoryMessage,
  subagents: Record<string, SubAgentRun>,
  toolResults: Map<string, string>,
): ContentBlock[] {
  const blocks: ContentBlock[] = []
  if (m.content) {
    blocks.push({ type: 'text', content: m.content })
  }

  let hasAgentCalls = false
  for (const tc of m.tool_calls || []) {
    const name = tc.function?.name || ''
    if (!name.startsWith('agent__')) continue
    hasAgentCalls = true
    const ref = m.subagent_refs?.find((r) => r.call_id === tc.id)
    const run = ref ? subagents[ref.span_id] : undefined
    if (run) {
      blocks.push(subagentCardFromRun(run))
      continue
    }
    let task: string
    try {
      task = JSON.parse(tc.function?.arguments || '{}')?.task ?? ''
    } catch {
      task = ''
    }
    blocks.push({
      type: 'subagent',
      span_id: tc.id,
      agent_name: '子智能体',
      task,
      status: 'done',
      steps: [],
      result: toolResults.get(tc.id),
    })
  }

  if (!hasAgentCalls && m.subagent_results?.length) {
    for (const r of m.subagent_results) {
      blocks.push({
        type: 'subagent',
        span_id: r.span_id,
        agent_name: r.agent_name,
        task: r.task,
        status: r.status === 'running' ? 'done' : r.status,
        steps: [],
        result: r.status === 'error' ? undefined : r.content,
        error: r.status === 'error' ? r.content : undefined,
      })
    }
  }
  return blocks
}

// call_id → 工具结果文本映射（用于旧格式兜底重建卡片）
export function buildToolResultMap(messages: HistoryMessage[]): Map<string, string> {
  const map = new Map<string, string>()
  for (const m of messages) {
    if (m.role === 'tool' && m.tool_call_id) map.set(m.tool_call_id, m.content || '')
  }
  return map
}
