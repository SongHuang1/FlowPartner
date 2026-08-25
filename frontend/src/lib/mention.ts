export interface MentionState {
  start: number
  query: string
}

// 从光标位置向前检测 @ 触发符（须位于行首或空白之后，query 不含空白）
export function detectMention(text: string, cursor: number): MentionState | null {
  let i = cursor - 1
  while (i >= 0) {
    const ch = text[i]
    if (ch === '@') {
      if (i === 0 || /\s/.test(text[i - 1])) {
        return { start: i, query: text.slice(i + 1, cursor) }
      }
      return null
    }
    if (/\s/.test(ch)) return null
    i--
  }
  return null
}
