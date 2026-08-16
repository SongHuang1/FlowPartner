import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import App from './App'
import { LockProvider } from '@/hooks/useLock'
import { SettingsProvider } from '@/hooks/useSettings'

vi.mock('@/lib/api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@/lib/api')>()
  return {
    ...actual,
    getHistoryList: vi.fn().mockResolvedValue([
      { session_id: 'sess_1', title: '测试对话', updated_at: 1000, message_count: 2 },
    ]),
    getHistorySession: vi.fn().mockResolvedValue({
      session_id: 'sess_1',
      messages: [
        { role: 'user', content: '你好' },
        { role: 'assistant', content: '你好！' },
      ],
    }),
  }
})

function renderApp() {
  return render(
    <SettingsProvider>
      <LockProvider>
        <App />
      </LockProvider>
    </SettingsProvider>
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

  it('suggested action buttons are enabled and functional', async () => {
    vi.useRealTimers()
    renderApp()

    const newChatButton = screen.getByRole('button', { name: '开始新对话' })
    const historyButton = screen.getByRole('button', { name: '查看历史' })

    expect(newChatButton).toBeEnabled()
    expect(historyButton).toBeEnabled()

    // 查看历史 → 显示历史列表
    fireEvent.click(historyButton)
    expect(await screen.findByText('历史对话')).toBeInTheDocument()
    expect(screen.getByText('测试对话')).toBeInTheDocument()

    // 点击历史会话 → 加载到聊天区
    fireEvent.click(screen.getByText('测试对话'))
    expect(await screen.findByText('你好！')).toBeInTheDocument()

    // 开始新对话 → 清空聊天区，回到欢迎页
    fireEvent.click(screen.getByRole('button', { name: '开始新对话' }))
    expect(screen.getByText('你好！我是 FlowPartner')).toBeInTheDocument()
  })
})
