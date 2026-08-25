import { useState, useCallback, useEffect, useRef } from 'react'
import { Copy, FileText } from 'lucide-react'
import { Tooltip } from '@/components/ui/tooltip'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import { renderToStaticMarkup } from 'react-dom/server'

interface MessageToolbarProps {
  content: string
}

type CopyState = 'idle' | 'copied' | 'failed'

async function copyMarkdown(text: string): Promise<boolean> {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    try {
      const textarea = document.createElement('textarea')
      textarea.value = text
      textarea.style.position = 'fixed'
      textarea.style.opacity = '0'
      document.body.appendChild(textarea)
      textarea.select()
      document.execCommand('copy')
      document.body.removeChild(textarea)
      return true
    } catch {
      return false
    }
  }
}

async function copyRichText(markdown: string): Promise<boolean> {
  const element = (
    <Markdown remarkPlugins={[remarkGfm]}>
      {markdown}
    </Markdown>
  )
  const html = renderToStaticMarkup(element)
  const plain = markdown

  try {
    if (typeof ClipboardItem !== 'undefined') {
      const item = new ClipboardItem({
        'text/html': new Blob([html], { type: 'text/html' }),
        'text/plain': new Blob([plain], { type: 'text/plain' }),
      })
      await navigator.clipboard.write([item])
      return true
    }
  } catch {
    // fallback below
  }

  try {
    const textarea = document.createElement('textarea')
    textarea.value = plain
    textarea.style.position = 'fixed'
    textarea.style.opacity = '0'
    document.body.appendChild(textarea)
    textarea.select()
    document.execCommand('copy')
    document.body.removeChild(textarea)
    return true
  } catch {
    return false
  }
}

export function MessageToolbar({ content }: MessageToolbarProps) {
  const [markdownState, setMarkdownState] = useState<CopyState>('idle')
  const [richTextState, setRichTextState] = useState<CopyState>('idle')
  const timersRef = useRef<ReturnType<typeof setTimeout>[]>([])

  useEffect(() => {
    return () => {
      timersRef.current.forEach(clearTimeout)
      timersRef.current = []
    }
  }, [])

  const resetAfter = useCallback((setter: (s: CopyState) => void) => {
    const timer = setTimeout(() => setter('idle'), 1500)
    timersRef.current.push(timer)
  }, [])

  const handleCopyMarkdown = async () => {
    const ok = await copyMarkdown(content)
    setMarkdownState(ok ? 'copied' : 'failed')
    resetAfter(setMarkdownState)
  }

  const handleCopyRichText = async () => {
    const ok = await copyRichText(content)
    setRichTextState(ok ? 'copied' : 'failed')
    resetAfter(setRichTextState)
  }

  const markdownLabel = markdownState === 'copied' ? '已复制' : markdownState === 'failed' ? '复制失败' : '复制 Markdown'
  const richTextLabel = richTextState === 'copied' ? '已复制' : richTextState === 'failed' ? '复制失败' : '复制富文本'

  return (
    <div className="flex items-center gap-2 mt-2 pt-2 border-t border-neutral-100">
      <Tooltip content="复制原始 Markdown 源码，适合粘贴到支持 Markdown 的编辑器（如 Typora、VS Code）">
        <button
          onClick={handleCopyMarkdown}
          className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-md transition-colors"
        >
          <Copy className="w-3.5 h-3.5" />
          {markdownLabel}
        </button>
      </Tooltip>
      <Tooltip content="复制带格式的富文本（HTML），适合粘贴到 Word、Notion、邮件等富文本编辑器">
        <button
          onClick={handleCopyRichText}
          className="inline-flex items-center gap-1.5 px-2.5 py-1.5 text-xs text-neutral-600 hover:text-neutral-900 hover:bg-neutral-100 rounded-md transition-colors"
        >
          <FileText className="w-3.5 h-3.5" />
          {richTextLabel}
        </button>
      </Tooltip>
    </div>
  )
}
