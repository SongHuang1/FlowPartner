import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import App from './App'

describe('App Integration', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders complete layout: title bar, activity bar, sidebar, chat area, status bar', () => {
    render(<App />)

    // TitleBar
    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
    expect(screen.getByText('UI Shell')).toBeInTheDocument()

    // ActivityBar
    expect(screen.getByRole('button', { name: '对话' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument()

    // Sidebar (default: conversation)
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()

    // ChatArea
    expect(screen.getByText('你好！我是你的 AI 助手 FlowPartner，有什么需要帮忙的吗？')).toBeInTheDocument()

    // StatusBar
    expect(screen.getByText('浏览器中运行 · 仅 UI 预览')).toBeInTheDocument()
  })

  it('sidebar switches to settings panel when clicking settings icon', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    expect(screen.getByText('设置功能即将推出')).toBeInTheDocument()
    expect(screen.queryByText('欢迎使用 FlowPartner')).not.toBeInTheDocument()
  })

  it('sidebar switches back to conversation panel when clicking conversation icon', () => {
    render(<App />)

    // Switch to settings
    fireEvent.click(screen.getByRole('button', { name: '设置' }))
    expect(screen.getByText('设置功能即将推出')).toBeInTheDocument()

    // Switch back to conversation
    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()
  })

  it('sidebar collapses when clicking close button', () => {
    render(<App />)

    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '收起侧边栏' }))

    expect(screen.queryByText('欢迎使用 FlowPartner')).not.toBeInTheDocument()
  })

  it('sidebar re-expands when clicking activity icon after collapse', () => {
    render(<App />)

    // Collapse sidebar
    fireEvent.click(screen.getByRole('button', { name: '收起侧边栏' }))
    expect(screen.queryByText('欢迎使用 FlowPartner')).not.toBeInTheDocument()

    // Click conversation icon to re-expand
    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()
  })

  it('clicking active view icon toggles sidebar visibility', () => {
    render(<App />)

    // Default: conversation selected, sidebar visible
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()

    // Click conversation icon again (same active view) → should collapse
    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    expect(screen.queryByText('欢迎使用 FlowPartner')).not.toBeInTheDocument()

    // Click conversation icon again → should expand
    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()
  })

  it('chat input works within the full app layout', () => {
    render(<App />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'integration test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(screen.getByText('integration test')).toBeInTheDocument()
  })

  it('ephemeral bubble disappears after 3 seconds in full app', () => {
    render(<App />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'timing test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(screen.getByText('timing test')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(3000)
    })

    expect(screen.queryByText('timing test')).not.toBeInTheDocument()
    expect(screen.getByText('你好！我是你的 AI 助手 FlowPartner，有什么需要帮忙的吗？')).toBeInTheDocument()
  })

  it('suggested action buttons in sidebar are disabled', () => {
    render(<App />)

    const newChatButton = screen.getByRole('button', { name: '开始新对话' })
    const historyButton = screen.getByRole('button', { name: '查看历史记录' })

    expect(newChatButton).toBeDisabled()
    expect(historyButton).toBeDisabled()
  })
})
