import { useCallback } from 'react'
import Markdown from 'react-markdown'
import remarkGfm from 'remark-gfm'
import remarkMath from 'remark-math'
import katex from 'katex'
import { Loader2 } from 'lucide-react'
import 'katex/dist/katex.min.css'
import type { Message } from '@/types'
import { MessageToolbar } from './MessageToolbar'

interface AssistantMessageProps {
  message: Message
  streamingContent?: string
}

export function AssistantMessage({ message, streamingContent }: AssistantMessageProps) {
  const isCompleted = message.status === 'completed'
  const isStreaming = message.status === 'streaming'
  const content = isStreaming && streamingContent ? streamingContent : message.content
  const subagentResults = message.subagent_results || []

  const handleLinkClick = useCallback((e: React.MouseEvent<HTMLAnchorElement>) => {
    e.preventDefault()
    const url = e.currentTarget.href
    window.flowPartner.openExternal(url)
  }, [])

  return (
    <div className="flex justify-start">
      <div className="w-full min-w-0">
        <div className="text-xs text-neutral-500 mb-1">FlowPartner</div>
        <div className="text-sm text-neutral-800 prose prose-sm max-w-none">
          <Markdown
            remarkPlugins={[remarkGfm, remarkMath]}
            components={{
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
                const { children } = props as { children?: React.ReactNode; [key: string]: unknown }
                // 检查是否是行间公式的 <pre><code class="math-display">
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
              code: (props) => {
                const { inline, className, children, ...rest } = props as { inline?: boolean; className?: string; children?: React.ReactNode; [key: string]: unknown }
                const text = String(children || '')

                // 行内公式（$...$）
                if (className && className.includes('math-inline')) {
                  try {
                    const html = katex.renderToString(text, { displayMode: false, throwOnError: false })
                    return <span className="katex-inline" dangerouslySetInnerHTML={{ __html: html }} />
                  } catch {
                    return <code className={className} {...rest}>{children}</code>
                  }
                }

                // 普通代码
                const hasLanguage = /language-(\w+)/.exec(className || '')
                if (inline || !hasLanguage) {
                  return (
                    <code className="bg-neutral-100 px-1 py-0.5 rounded text-pink-600 text-[0.875em] font-mono" {...rest}>
                      {children}
                    </code>
                  )
                }
                return (
                  <div className="relative my-3">
                    {className && (
                      <div className="absolute top-0 left-0 px-3 py-1 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono z-10">
                        {className.replace('language-', '')}
                      </div>
                    )}
                    <pre className="bg-neutral-900 text-neutral-100 pt-8 p-4 rounded-lg overflow-x-auto max-h-[400px]">
                      <code className="font-mono text-sm leading-relaxed" {...rest}>
                        {children}
                      </code>
                    </pre>
                  </div>
                )
              },
            }}
          >
            {content}
          </Markdown>
        </div>
        {/* 子智能体调用结果 */}
        {subagentResults.length > 0 && (
          <div className="mt-3 space-y-2">
            {subagentResults.map((result) => (
              <div key={result.span_id} className="rounded-lg border border-neutral-200 bg-neutral-50 overflow-hidden">
                <div className="flex items-center gap-2 px-3 py-2 bg-neutral-100 border-b border-neutral-200">
                  <span className="text-sm font-medium text-neutral-700">{result.agent_name}</span>
                  <span className="text-xs text-neutral-400 truncate flex-1">{result.task}</span>
                  {result.status === 'running' && (
                    <span className="text-xs text-blue-600 flex items-center gap-1">
                      <Loader2 className="w-3 h-3 animate-spin" />
                      执行中
                    </span>
                  )}
                  {result.status === 'done' && (
                    <span className="text-xs text-green-600">已完成</span>
                  )}
                  {result.status === 'error' && (
                    <span className="text-xs text-red-500">失败</span>
                  )}
                </div>
                {result.content && (
                  <div className="px-3 py-2 text-sm text-neutral-800 prose prose-sm max-w-none">
                    <Markdown
                      remarkPlugins={[remarkGfm, remarkMath]}
                      components={{
                        pre: (props) => {
                          const { children } = props as { children?: React.ReactNode; [key: string]: unknown }
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
                        code: (props) => {
                          const { inline, className, children, ...rest } = props as { inline?: boolean; className?: string; children?: React.ReactNode; [key: string]: unknown }
                          if (className && className.includes('math-inline')) {
                            const text = String(children || '')
                            try {
                              const html = katex.renderToString(text, { displayMode: false, throwOnError: false })
                              return <span className="katex-inline" dangerouslySetInnerHTML={{ __html: html }} />
                            } catch {
                              return <code className={className} {...rest}>{children}</code>
                            }
                          }
                          const hasLanguage = /language-(\w+)/.exec(className || '')
                          if (inline || !hasLanguage) {
                            return <code className="bg-neutral-100 px-1 py-0.5 rounded text-pink-600 text-[0.875em] font-mono" {...rest}>{children}</code>
                          }
                          return (
                            <div className="relative my-2">
                              {className && (
                                <div className="absolute top-0 left-0 px-2 py-0.5 text-xs text-neutral-500 bg-neutral-200 rounded-tl rounded-br font-mono z-10">
                                  {className.replace('language-', '')}
                                </div>
                              )}
                              <pre className="bg-neutral-900 text-neutral-100 pt-6 p-3 rounded-lg overflow-x-auto max-h-[300px]">
                                <code className="font-mono text-xs leading-relaxed" {...rest}>{children}</code>
                              </pre>
                            </div>
                          )
                        },
                      }}
                    >
                      {result.content}
                    </Markdown>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
        {isCompleted && <MessageToolbar content={message.content} />}
      </div>
    </div>
  )
}
