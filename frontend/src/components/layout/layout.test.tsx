import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { TitleBar } from './TitleBar'
import { ActivityBar } from './ActivityBar'
import { Sidebar } from './Sidebar'
import { StatusBar } from './StatusBar'

describe('TitleBar', () => {
  it('renders FlowPartner name and UI Shell indicator', () => {
    render(<TitleBar />)
    expect(screen.getByText('FlowPartner')).toBeInTheDocument()
    expect(screen.getByText('UI Shell')).toBeInTheDocument()
  })
})

describe('ActivityBar', () => {
  it('renders two icon buttons', () => {
    const onSelect = vi.fn()
    render(<ActivityBar activeView="conversation" onSelect={onSelect} />)
    expect(screen.getByRole('button', { name: '对话' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: '设置' })).toBeInTheDocument()
  })

  it('calls onSelect when clicking an icon', () => {
    const onSelect = vi.fn()
    render(<ActivityBar activeView="conversation" onSelect={onSelect} />)
    fireEvent.click(screen.getByRole('button', { name: '设置' }))
    expect(onSelect).toHaveBeenCalledWith('settings')
  })
})

describe('Sidebar', () => {
  it('renders conversation panel when activeView is conversation', () => {
    render(<Sidebar visible={true} activeView="conversation" onClose={() => {}} />)
    expect(screen.getByText('欢迎使用 FlowPartner')).toBeInTheDocument()
  })

  it('renders settings panel when activeView is settings', () => {
    render(<Sidebar visible={true} activeView="settings" onClose={() => {}} />)
    expect(screen.getByText('模型')).toBeInTheDocument()
  })

  it('calls onClose when clicking close button', () => {
    const onClose = vi.fn()
    render(<Sidebar visible={true} activeView="conversation" onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: '收起侧边栏' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('has zero width when not visible', () => {
    const { container } = render(<Sidebar visible={false} activeView="conversation" onClose={() => {}} />)
    const el = container.querySelector('[data-testid="sidebar-panel"]')
    expect(el?.className).toContain('w-0')
  })

  it('always renders the sidebar panel element even when not visible', () => {
    const { container } = render(<Sidebar visible={false} activeView="conversation" onClose={() => {}} />)
    const el = container.querySelector('[data-testid="sidebar-panel"]')
    expect(el).not.toBeNull()
  })

  it('has w-64 width when visible', () => {
    const { container } = render(<Sidebar visible={true} activeView="conversation" onClose={() => {}} />)
    const el = container.querySelector('[data-testid="sidebar-panel"]')
    expect(el?.className).toContain('w-64')
    expect(el?.className).not.toContain('w-0')
  })

  it('sets aria-hidden=true when not visible', () => {
    const { container } = render(<Sidebar visible={false} activeView="conversation" onClose={() => {}} />)
    const el = container.querySelector('[data-testid="sidebar-panel"]')
    expect(el).toHaveAttribute('aria-hidden', 'true')
  })

  it('sets aria-hidden=false when visible', () => {
    const { container } = render(<Sidebar visible={true} activeView="conversation" onClose={() => {}} />)
    const el = container.querySelector('[data-testid="sidebar-panel"]')
    expect(el).toHaveAttribute('aria-hidden', 'false')
  })

  it('has overflow-hidden class for transition clipping', () => {
    const { container } = render(<Sidebar visible={false} activeView="conversation" onClose={() => {}} />)
    const el = container.querySelector('[data-testid="sidebar-panel"]')
    expect(el?.className).toContain('overflow-hidden')
  })
})

describe('StatusBar', () => {
  const originalDescriptor = Object.getOwnPropertyDescriptor(window, 'flowPartner')

  afterEach(() => {
    // Restore original descriptor after each test
    if (originalDescriptor) {
      Object.defineProperty(window, 'flowPartner', originalDescriptor)
    } else {
      delete (window as unknown as Record<string, unknown>)['flowPartner']
    }
  })

  it('renders correct preview mode text', () => {
    delete (window as unknown as Record<string, unknown>)['flowPartner']
    render(<StatusBar />)
    expect(screen.getByText('浏览器中运行 · 仅 UI 预览')).toBeInTheDocument()
  })

  it('renders desktop text when running in Electron', () => {
    Object.defineProperty(window, 'flowPartner', {
      value: { platform: 'win32', version: '1.0.0' },
      writable: true,
      configurable: true,
    })
    render(<StatusBar />)
    expect(screen.getByText('桌面端 · FlowPartner')).toBeInTheDocument()
  })

  it('renders preview text when window.flowPartner is undefined', () => {
    delete (window as unknown as Record<string, unknown>)['flowPartner']
    render(<StatusBar />)
    expect(screen.getByText('浏览器中运行 · 仅 UI 预览')).toBeInTheDocument()
    expect(screen.queryByText('桌面端 · FlowPartner')).not.toBeInTheDocument()
  })
})
