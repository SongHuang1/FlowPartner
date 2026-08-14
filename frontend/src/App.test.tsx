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
    expect(screen.getByRole('button', { name: 'Chat' })).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Settings' })).toBeInTheDocument()
    expect(screen.getByText('Welcome to FlowPartner')).toBeInTheDocument()
    expect(screen.getByText('Running in browser · UI preview only')).toBeInTheDocument()
  })

  it('sidebar switches to settings panel when clicking settings icon', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: 'Settings' }))

    expect(screen.getByText('API Settings')).toBeInTheDocument()
    expect(screen.queryByText('Welcome to FlowPartner')).not.toBeInTheDocument()
  })

  it('sidebar switches back to conversation panel when clicking conversation icon', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: 'Settings' }))
    expect(screen.getByText('API Settings')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Chat' }))
    expect(screen.getByText('Welcome to FlowPartner')).toBeInTheDocument()
  })

  it('sidebar collapses when clicking close button', () => {
    render(<App />)

    expect(screen.getByText('Welcome to FlowPartner')).toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }))

    const sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')
  })

  it('sidebar re-expands when clicking activity icon after collapse', () => {
    render(<App />)

    fireEvent.click(screen.getByRole('button', { name: 'Collapse sidebar' }))
    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')

    fireEvent.click(screen.getByRole('button', { name: 'Chat' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')
  })

  it('clicking active view icon toggles sidebar visibility', () => {
    render(<App />)

    let sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')

    fireEvent.click(screen.getByRole('button', { name: 'Chat' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-0')

    fireEvent.click(screen.getByRole('button', { name: 'Chat' }))
    sidebar = document.querySelector('[data-testid="sidebar-panel"]')
    expect(sidebar?.className).toContain('w-64')
  })

  it('suggested action buttons in sidebar are disabled', () => {
    render(<App />)

    const newChatButton = screen.getByRole('button', { name: 'Start new chat' })
    const historyButton = screen.getByRole('button', { name: 'View history' })

    expect(newChatButton).toBeDisabled()
    expect(historyButton).toBeDisabled()
  })
})
