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
    expect(screen.getByText('设置功能即将推出')).toBeInTheDocument()
  })

  it('calls onClose when clicking close button', () => {
    const onClose = vi.fn()
    render(<Sidebar visible={true} activeView="conversation" onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: '收起侧边栏' }))
    expect(onClose).toHaveBeenCalledTimes(1)
  })

  it('renders null when not visible', () => {
    const { container } = render(<Sidebar visible={false} activeView="conversation" onClose={() => {}} />)
    expect(container.firstChild).toBeNull()
  })
})

describe('StatusBar', () => {
  it('renders correct preview mode text', () => {
    render(<StatusBar />)
    expect(screen.getByText('浏览器中运行 · 仅 UI 预览')).toBeInTheDocument()
  })
})
