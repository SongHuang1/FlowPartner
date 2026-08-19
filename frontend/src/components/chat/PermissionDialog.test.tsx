import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { PermissionDialog } from './PermissionDialog'
import type { PermissionRequestPayload } from '@/types'

const mockRequest: PermissionRequestPayload = {
  request_id: 'req-test-123',
  tool: 'read',
  path: '/tmp/secret.txt',
  operation: '读取',
  detail: 'Agent 想要读取路径 /tmp/secret.txt',
}

describe('PermissionDialog', () => {
  it('renders the dialog with correct operation and path', () => {
    render(<PermissionDialog request={mockRequest} onDecision={vi.fn()} />)

    expect(screen.getByText('权限申请')).toBeInTheDocument()
    expect(screen.getByText('读取文件')).toBeInTheDocument()
    expect(screen.getByText('/tmp/secret.txt')).toBeInTheDocument()
    expect(screen.getByText('read', { selector: '.font-mono' })).toBeInTheDocument()
  })

  it('calls onDecision with allow when Allow button is clicked', () => {
    const onDecision = vi.fn()
    render(<PermissionDialog request={mockRequest} onDecision={onDecision} />)

    fireEvent.click(screen.getByRole('button', { name: '允许一次' }))
    expect(onDecision).toHaveBeenCalledWith('allow')
  })

  it('calls onDecision with deny when Deny button is clicked', () => {
    const onDecision = vi.fn()
    render(<PermissionDialog request={mockRequest} onDecision={onDecision} />)

    fireEvent.click(screen.getByRole('button', { name: '拒绝' }))
    expect(onDecision).toHaveBeenCalledWith('deny')
  })

  it('calls onDecision with deny when Escape is pressed', () => {
    const onDecision = vi.fn()
    render(<PermissionDialog request={mockRequest} onDecision={onDecision} />)

    fireEvent.keyDown(document, { key: 'Escape' })
    expect(onDecision).toHaveBeenCalledWith('deny')
  })

  it('calls onDecision with deny when clicking the backdrop', () => {
    const onDecision = vi.fn()
    render(<PermissionDialog request={mockRequest} onDecision={onDecision} />)

    const backdrop = document.querySelector('.fixed.inset-0.bg-black\\/50')
    expect(backdrop).toBeInTheDocument()
    fireEvent.click(backdrop!)
    expect(onDecision).toHaveBeenCalledWith('deny')
  })

  it('shows session allow button when scope_options includes session', () => {
    const requestWithScope: PermissionRequestPayload = {
      ...mockRequest,
      scope_options: ['once', 'session'],
    }
    render(<PermissionDialog request={requestWithScope} onDecision={vi.fn()} />)

    expect(screen.getByRole('button', { name: '本次会话允许' })).toBeInTheDocument()
    expect(screen.getByText('"本次会话允许"将在同一会话内自动放行相同工具和路径，跨会话仍需授权。')).toBeInTheDocument()
  })

  it('does not show session allow button without scope_options', () => {
    render(<PermissionDialog request={mockRequest} onDecision={vi.fn()} />)

    expect(screen.queryByRole('button', { name: '本次会话允许' })).not.toBeInTheDocument()
  })

  it('calls onDecision with allow_session when session allow button is clicked', () => {
    const onDecision = vi.fn()
    const requestWithScope: PermissionRequestPayload = {
      ...mockRequest,
      scope_options: ['once', 'session'],
    }
    render(<PermissionDialog request={requestWithScope} onDecision={onDecision} />)

    fireEvent.click(screen.getByRole('button', { name: '本次会话允许' }))
    expect(onDecision).toHaveBeenCalledWith('allow_session')
  })

  it('renders without crashing with different operation types', () => {
    const writeRequest: PermissionRequestPayload = {
      ...mockRequest,
      tool: 'write',
      operation: '写入',
      path: '/etc/config.json',
    }
    render(<PermissionDialog request={writeRequest} onDecision={vi.fn()} />)

    expect(screen.getByText('写入文件')).toBeInTheDocument()
    expect(screen.getByText('/etc/config.json')).toBeInTheDocument()
  })
})
