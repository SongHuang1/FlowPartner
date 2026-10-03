import type { ContentBlock } from '@/types'

export interface ToolExecution {
  /** 工具自身是否执行成功 */
  success: boolean
  result: string
  errorCode: string
  toolName: string
  /** 原始 arguments（JSON 字符串或已解析对象） */
  rawArgs: string
  args: Record<string, unknown>
}

/** 解析 item/completed 的 payload：后端把 success/result/tool_name/arguments 打包在此 */
export function parseToolPayload(payload: string | undefined): ToolExecution | null {
  if (!payload) return null
  let parsed: unknown
  try {
    parsed = JSON.parse(payload)
  } catch {
    return null
  }
  if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) return null
  const p = parsed as Record<string, unknown>

  let args: Record<string, unknown> = {}
  let rawArgs = ''
  const argVal = p.arguments
  if (typeof argVal === 'string' && argVal.trim()) {
    rawArgs = argVal
    try {
      const a = JSON.parse(argVal)
      if (a && typeof a === 'object' && !Array.isArray(a)) args = a as Record<string, unknown>
    } catch { /* 保持 rawArgs 原样 */
    }
  } else if (argVal && typeof argVal === 'object' && !Array.isArray(argVal)) {
    args = argVal as Record<string, unknown>
    rawArgs = JSON.stringify(args)
  }

  return {
    success: p.success === true,
    result: typeof p.result === 'string' ? p.result : '',
    errorCode: typeof p.error_code === 'string' ? p.error_code : '',
    toolName: typeof p.tool_name === 'string' ? p.tool_name : '',
    rawArgs,
    args,
  }
}

function firstString(args: Record<string, unknown>, keys: string[]): string {
  for (const k of keys) {
    const v = args[k]
    if (typeof v === 'string' && v.trim()) return v.trim()
  }
  return ''
}

/**
 * 生成"执行了什么"的一句话摘要。
 * read → 目标文件路径；bash → 命令本身；write/edit → 目标路径；
 * 其余工具 → 首个可识别的路径/命令参数，都没有则留空。
 */
export function describeToolCall(toolName: string, args: Record<string, unknown>): string {
  switch (toolName) {
    case 'read':
      return firstString(args, ['path', 'file_path', 'filePath'])
    case 'bash':
      return firstString(args, ['command', 'cmd'])
    case 'write':
    case 'edit':
    case 'trash':
    case 'purge':
      return firstString(args, ['path', 'file_path', 'filePath'])
    default:
      return firstString(args, ['path', 'file_path', 'filePath', 'command', 'cmd', 'query', 'text'])
  }
}

/**
 * 从 item/completed 构造 tool_call block。
 * status 表示"调用是否成功拿到结果"，success 表示"工具执行是否成功"。
 */
export function buildToolCallBlock(
  callId: string,
  payload: string | undefined,
  fallbackName: string,
): ContentBlock & { type: 'tool_call' } | null {
  const exec = parseToolPayload(payload)
  if (!exec) return null
  const toolName = exec.toolName || fallbackName
  const summary = describeToolCall(toolName, exec.args)
  return {
    type: 'tool_call',
    call_id: callId,
    tool_name: toolName,
    arguments: exec.rawArgs,
    status: 'done',
    success: exec.success,
    summary,
    result: exec.result,
    error: exec.success ? undefined : exec.result || exec.errorCode,
  }
}

/** 提取结果文本的最后一句片段，用于子智能体收起态单行预览 */
export function tailSnippet(text: string, maxChars = 120): string {
  const flat = text.replace(/\s+/g, ' ').trim()
  if (!flat) return ''
  if (flat.length <= maxChars) return flat
  // 截到 maxChars 内的最后一个句读/标点，避免半句截断
  const window = flat.slice(flat.length - maxChars)
  const cut = Math.max(
    window.lastIndexOf('。'), window.lastIndexOf('. '),
    window.lastIndexOf('！'), window.lastIndexOf('？'),
    window.lastIndexOf(';'), window.lastIndexOf('；'),
  )
  if (cut > 0) {
    // 标点后若无实义内容（如只剩一个句号），退回整段并去掉尾部标点
    const after = window.slice(cut + 1).trim()
    if (after && /[^\s\p{P}]/u.test(after)) return after
    return window.replace(/[\s\p{P}]+$/u, '')
  }
  return window
}
