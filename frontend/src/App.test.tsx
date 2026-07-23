import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
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

    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
    expect(screen.getByText('UI Shell')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '对话' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument()
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()
    expect(screen.getByText('浏览器中运行 · 仅 UI 预览')).toBeInTheDocument()
  })

  it('sidebar switches to settings panel when clicking settings icon', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    expect(screen.getByText('API 配置')).toBeInTheDocument()
    expect(screen.queryByText('欢迎使用 FlowPartner')).not.toBeInTheDocument()
  })

  it('sidebar switches back to conversation panel when clicking conversation icon', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: '设置' }))
    expect(screen.getByText('API 配置')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()
  })

  it('sidebar collapses when clicking close button', () => {
    render(<App />)

    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '收起侧边栏' }))

    const sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')
  })

  it('sidebar re-expands when clicking activity icon after collapse', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: '收起侧边栏' }))
    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')

    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')
  })

  it('clicking active view icon toggles sidebar visibility', () => {
    render(<App />)

    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')

    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')

    fireEvent.click(screen.getByRole('button', { name: '对话' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')
  })

  it('suggested action buttons in sidebar are disabled', () => {
    render(<App />)

    const newChatButton = screen.getByRole('button', { name: '开始新对话' })
    const historyButton = screen.getByRole('button', { name: '查看历史记录' })

    expect(newChatButton).toBeDisabled()
    expect(historyButton).toBeDisabled()
  })
})
