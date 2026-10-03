import { useState, useCallback } from 'react'
import Markdown from 'react-markdown'
import type { Components } from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import katex from 'katex'
import { Loader2, CheckCircle2, XCircle } from 'lucide-react'
import 'katex/dist/katex.min.css'
import type { Message, ContentBlock } from '@/types'
import { tailSnippet } from '@/lib/toolcall'
import { MessageToolbar } from './MessageToolbar'

interface AssistantMessageProps {
  message: Message
  streamingContent?: string
}

export function AssistantMessage({ message, streamingContent }: AssistantMessageProps) {
  const isCompleted = message.status === 'completed'
  const blocks = message.content_blocks

  // 无内容块时回退到扁平 content（历史消息、直接构造的消息）；
  // 有内容块时仅从中取 text，保证文本与工具/子智能体块的时序不被破坏。
  const effectiveBlocks: ContentBlock[] =
    blocks && blocks.length > 0
      ? blocks
      : message.content
        ? [{ type: 'text', content: message.content }]
        : []

  const liveText = streamingContent || message.content
  const copyContent = (effectiveBlocks.some((b) => b.type === 'text')
    ? effectiveBlocks.filter((b) => b.type === 'text').map((b) => b.content).join('')
    : liveText) || ''

  const [expandedAgent, setExpandedAgent] = useState<string | null>(null)

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    const url = e.currentTarget.href
    window.flowPartner.openExternal(url)
  }, [])

  const mdComponents: Components = {
    a: (props) => (
      <a
        {...props}
        onClick={handleLinkClick}
        target="_blank"
        rel="noopener noreferrer"
        className="text-blue-600 hover:underline"
      />
    ),
    pre: (props) => {
      const { children } = props
      const child = children as { props?: { className?: string; children?: React.ReactNode } } | undefined
      if (child?.props?.className?.includes('math-display')) {
        const text = String(child.props.children || '')
        try {
          const html = katex.renderToString(text, { displayMode: true, throwOnError: false })
          return <div className="katex-block" dangerouslySetInnerHTML={{ __html: html }} />
        } catch {
          return <pre {...props} />
        }
      }
      return <pre {...props} />
    },
    code: ({ className, children, ...rest }) => {
      const hasLanguage = /language-(\w+)/.exec(className || '')
      if (!hasLanguage) {
        return <code className="bg-neutral-100 px-1 py-0.5 rounded text-pink-600 text-[0.875em] font-mono" {...rest}>{children}</code>
      }
      return (
        <div className="relative my-3">
          {className && (
            <div className="absolute top-0 left-0 px-3 py-1 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono z-10">
              {className.replace('language-', '')}
            </div>
          )}
          <pre className="bg-neutral-900 text-neutral-100 pt-8 p-4 rounded-lg overflow-x-auto max-h-[400px]">
            <code className="font-mono text-sm leading-relaxed" {...rest}>{children}</code>
          </pre>
        </div>
      )
    },
  }

  const renderSubagentBlock = (block: Extract<ContentBlock, { type: 'subagent' }>, idx: number) => {
    const key = block.span_id || `subagent_${idx}`
    const isExpanded = expandedAgent === block.span_id
    const body = block.error || block.result || ''
    const canToggle = !!body

    return (
      <div key={key} className="text-sm">
        {/* 名字加粗 + 灰色摘要，跟在正文流里，不占固定列宽 */}
        <div className="flex items-start gap-1.5">
          {block.status === 'running' && (
            <Loader2 className="mt-1 w-3 h-3 shrink-0 animate-spin text-blue-500" />
          )}
          <span className="font-semibold text-neutral-800 shrink-0">{block.agent_name}</span>

          {block.status === 'running' && !body && (
            <span className="text-neutral-500">正在执行…</span>
          )}

          {block.status === 'error' && !body && (
            <span className="text-neutral-500">执行失败</span>
          )}

          {body && (
            isExpanded ? (
              <span className="text-neutral-500 min-w-0 flex-1 whitespace-pre-wrap break-words">
                {body}
              </span>
            ) : (
              <span className="text-neutral-500 min-w-0 flex-1 inline-block align-top whitespace-pre-wrap break-words max-h-[3em] overflow-y-auto">
                {tailSnippet(body)}
              </span>
            )
          )}

          {canToggle && (
            <button
              type="button"
              onClick={() => setExpandedAgent(isExpanded ? null : (block.span_id || String(idx)))}
              className="shrink-0 text-xs text-blue-500 hover:underline mt-0.5"
            >
              {isExpanded ? '收起' : '展开'}
            </button>
          )}
        </div>
      </div>
    )
  }

  const renderToolCallBlock = (block: Extract<ContentBlock, { type: 'tool_call' }>, idx: number) => {
    const key = block.call_id || `tool_${idx}`
    const isRunning = block.status === 'running'
    // 调用失败 ≠ 执行失败：status 反映能否拿到结果，success 反映工具自身是否成功
    const callFailed = block.status === 'error'
    const execFailed = !isRunning && !callFailed && block.success === false

    return (
      <div key={key} className="flex items-start gap-1.5 text-sm">
        {isRunning ? (
          <Loader2 className="mt-1 w-3.5 h-3.5 shrink-0 animate-spin text-blue-500" />
        ) : callFailed || execFailed ? (
          <XCircle className="mt-1 w-3.5 h-3.5 shrink-0 text-red-500" />
        ) : (
          <CheckCircle2 className="mt-1 w-3.5 h-3.5 shrink-0 text-green-500" />
        )}

        <span className="min-w-0 flex-1">
          <span className="text-neutral-800">
            {block.tool_name || '工具'}
            {block.summary && <span className="text-neutral-500"> {block.summary}</span>}
          </span>

          <span className="text-xs text-neutral-400 ml-1.5">
            {isRunning ? '执行中' : callFailed ? '调用失败' : execFailed ? '执行失败' : '执行成功'}
          </span>

          {/* 结果文本本身就是用户需要的输出，不再展示原始 JSON 信封 */}
          {!isRunning && block.result && !execFailed && (
            <span className="block text-neutral-500 text-xs font-mono mt-0.5 max-h-24 overflow-y-auto whitespace-pre-wrap break-all">
              {block.result}
            </span>
          )}
          {!isRunning && (execFailed || callFailed) && (block.error || block.result) && (
            <span className="block text-red-500 text-xs font-mono mt-0.5 max-h-24 overflow-y-auto whitespace-pre-wrap break-all">
              {block.error || block.result}
            </span>
          )}
        </span>
      </div>
    )
  }

  const renderBlock = (block: ContentBlock, idx: number) => {
    switch (block.type) {
      case 'text':
        if (!block.content?.trim()) return null
        return (
          <div key={idx} className="text-sm text-neutral-800 prose prose-sm max-w-none">
            <Markdown remarkPlugins={[remarkGfm, remarkMath]} components={mdComponents}>
              {block.content}
            </Markdown>
          </div>
        )
      case 'tool_call':
        return renderToolCallBlock(block, idx)
      case 'subagent':
        return renderSubagentBlock(block, idx)
      default:
        return null
    }
  }

  return (
    <div className="flex justify-start">
      <div className="w-full min-w-0">
        <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        <div className="space-y-2">
          {effectiveBlocks.map((block, i) => renderBlock(block, i))}
        </div>
        {isCompleted && <MessageToolbar content={copyContent} />}
      </div>
    </div>
  )
}
