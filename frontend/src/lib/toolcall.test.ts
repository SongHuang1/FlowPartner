import { describe, it, expect } from 'vitest'
import { parseToolPayload, describeToolCall, buildToolCallBlock, tailSnippet } from './toolcall'

describe('parseToolPayload', () => {
  it('extracts success, result, tool name and arguments', () => {
    const payload = JSON.stringify({
      success: true,
      result: 'file contents here',
      error_code: '',
      tool_name: 'read',
      arguments: { path: 'src/main.ts' },
    })
    const exec = parseToolPayload(payload)
    expect(exec).not.toBeNull()
    expect(exec!.success).toBe(true)
    expect(exec!.result).toBe('file contents here')
    expect(exec!.toolName).toBe('read')
    expect(exec!.args).toEqual({ path: 'src/main.ts' })
  })

  it('reports failure without throwing', () => {
    const payload = JSON.stringify({ success: false, result: '路径越界', error_code: 'PATH_OUTSIDE', tool_name: 'read' })
    const exec = parseToolPayload(payload)
    expect(exec!.success).toBe(false)
    expect(exec!.result).toBe('路径越界')
    expect(exec!.errorCode).toBe('PATH_OUTSIDE')
  })

  it('returns null for malformed payload', () => {
    expect(parseToolPayload('not json')).toBeNull()
    expect(parseToolPayload(undefined)).toBeNull()
    expect(parseToolPayload('[1,2]')).toBeNull()
  })

  it('accepts arguments given as a JSON string', () => {
    const payload = JSON.stringify({ success: true, result: 'ok', tool_name: 'bash', arguments: '{"command":"ls -la"}' })
    const exec = parseToolPayload(payload)
    expect(exec!.args).toEqual({ command: 'ls -la' })
  })
})

describe('describeToolCall', () => {
  it('describes read by target file', () => {
    expect(describeToolCall('read', { path: 'src/main.ts' })).toBe('src/main.ts')
  })

  it('describes bash by the command itself', () => {
    expect(describeToolCall('bash', { command: 'ls -la /tmp' })).toBe('ls -la /tmp')
  })

  it('describes write and edit by path', () => {
    expect(describeToolCall('write', { path: 'a.txt' })).toBe('a.txt')
    expect(describeToolCall('edit', { path: 'b.txt' })).toBe('b.txt')
  })

  it('falls back to the first recognisable argument', () => {
    expect(describeToolCall('agent__x', { task: '翻译' })).toBe('')
    expect(describeToolCall('grep', { path: 'c.txt' })).toBe('c.txt')
  })
})

describe('buildToolCallBlock', () => {
  it('builds a done block carrying success and summary', () => {
    const payload = JSON.stringify({ success: true, result: 'ok', tool_name: 'bash', arguments: { command: 'ls' } })
    const block = buildToolCallBlock('c1', payload, '工具')
    expect(block).not.toBeNull()
    expect(block!.type).toBe('tool_call')
    expect(block!.status).toBe('done')
    expect(block!.success).toBe(true)
    expect(block!.summary).toBe('ls')
    expect(block!.tool_name).toBe('bash')
  })

  it('distinguishes call success from execution failure', () => {
    const payload = JSON.stringify({ success: false, result: '命令返回非零', error_code: 'TOOL_ERROR', tool_name: 'bash', arguments: { command: 'ls' } })
    const block = buildToolCallBlock('c1', payload, '工具')
    expect(block!.status).toBe('done')
    expect(block!.success).toBe(false)
    expect(block!.error).toBe('命令返回非零')
  })

  it('returns null when payload is unusable', () => {
    expect(buildToolCallBlock('c1', undefined, '工具')).toBeNull()
  })
})

describe('tailSnippet', () => {
  it('keeps short text as is', () => {
    expect(tailSnippet('翻译完成')).toBe('翻译完成')
  })

  it('cuts back to the previous punctuation for long text', () => {
    const text = '第一句话。第二句话。第三句话内容很长需要截断。'
    const out = tailSnippet(text, 12)
    expect(out.length).toBeGreaterThan(0)
    expect(out.length).toBeLessThanOrEqual(12)
    // 取自尾部，且不以标点结尾（避免只剩孤立标点）
    expect(text.endsWith(out) || text.includes(out)).toBe(true)
    expect(/[\s\p{P}]$/u.test(out)).toBe(false)
  })

  it('never returns a lone punctuation mark', () => {
    const out = tailSnippet('结论：完成了。', 4)
    expect(out).not.toBe('。')
    expect(/[^\s\p{P}]/u.test(out)).toBe(true)
  })

  it('collapses whitespace', () => {
    expect(tailSnippet('a\n\n  b')).toBe('a b')
  })

  it('returns empty for blank input', () => {
    expect(tailSnippet('   ')).toBe('')
  })
})
