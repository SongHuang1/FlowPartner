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
    expect(screen.getByRole('button', { name: 'Chat' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Settings' })).toBeInTheDocument()
  })

  it('calls onSelect when clicking an icon', () => {
    const onSelect = vi.fn()
    render(<ActivityBar activeView="conversation" onSelect={onSelect} />)
    fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    expect(onSelect).toHaveBeenCalledWith('settings')
  })
})

describe('Sidebar', () => {
  it('renders conversation panel when activeView is conversation', () => {
    render(<Sidebar visible={true} activeView="conversation" onClose={() => {}} />)
    expect(screen.getByText('Welcome to FlowPartner')).toBeInTheDocument()
  })

  it('renders settings panel when activeView is settings', () => {
    render(<Sidebar visible={true} activeView="settings" onClose={() => {}} />)
    expect(screen.getByText('API Settings')).toBeInTheDocument()
  })

  it('calls onClose when clicking close button', () => {
    const onClose = vi.fn()
    render(<Sidebar visible={true} activeView="conversation" onClose={onClose} />)
    fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }))
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
    expect(screen.getByText('Running in browser · UI preview only')).toBeInTheDocument()
  })

  it('renders desktop text when running in Electron', () => {
    Object.defineProperty(window, 'flowPartner', {
      value: { platform: 'win32', version: '1.0.0' },
      writable: true,
      configurable: true,
    })
    render(<StatusBar />)
    expect(screen.getByText('Desktop · FlowPartner')).toBeInTheDocument()
  })

  it('renders preview text when window.flowPartner is undefined', () => {
    delete (window as unknown as Record<string, unknown>)['flowPartner']
    render(<StatusBar />)
    expect(screen.getByText('Running in browser · UI preview only')).toBeInTheDocument()
    expect(screen.queryByText('Desktop · FlowPartner')).not.toBeInTheDocument()
  })
})
