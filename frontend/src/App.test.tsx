import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import App from './App'
import { AllProviders } from './test-setup'

function renderApp() {
  return render(
    <AllProviders>
      <App />
    </AllProviders>
  )
}

describe('App Integration', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders complete layout: title bar, activity bar, sidebar, chat area, status bar', () => {
    renderApp()

    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
    expect(screen.getByText('界面框架')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '聊天' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument()
    expect(screen.getByText('聊天记录')).toBeInTheDocument()
    expect(screen.getByText((content) => content.includes('FlowPartner') && content.includes('版'))).toBeInTheDocument()
  })

  it('settings icon opens modal, not sidebar', () => {
    renderApp()

    fireEvent.click(screen.getByRole('button', { name: '设置' }))

    expect(screen.getByText('API 配置')).toBeInTheDocument()
    expect(screen.getByText('聊天记录')).toBeInTheDocument()
  })

  it('chat icon toggles sidebar visibility', () => {
    renderApp()

    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar).toHaveClass('w-64')

    fireEvent.click(screen.getByRole('button', { name: '聊天' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar).toHaveClass('w-0')

    fireEvent.click(screen.getByRole('button', { name: '聊天' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar).toHaveClass('w-64')
  })

  it('sidebar collapses when clicking close button', () => {
    renderApp()

    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '收起侧栏' }))

    const sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')
  })

  it('sidebar re-expands when clicking activity icon after collapse', () => {
    renderApp()

    fireEvent.click(screen.getByRole('button', { name: '收起侧栏' }))
    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')

    fireEvent.click(screen.getByRole('button', { name: '聊天' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')
  })

  it('clicking active view icon toggles sidebar visibility', () => {
    renderApp()

    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')

    fireEvent.click(screen.getByRole('button', { name: '聊天' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')

    fireEvent.click(screen.getByRole('button', { name: '聊天' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')
  })

  it('suggested action buttons in sidebar are disabled', () => {
    renderApp()

    const newChatButton = screen.getByRole('button', { name: '开始新对话' })
    const historyButton = screen.getByRole('button', { name: '查看历史' })

    expect(newChatButton).toBeDisabled()
    expect(historyButton).toBeDisabled()
  })
})
