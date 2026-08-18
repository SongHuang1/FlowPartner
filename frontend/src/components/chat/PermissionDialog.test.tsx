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

    fireEvent.click(screen.getByRole('button', { name: '允许' }))
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

    // The backdrop is the fixed inset-0 bg-black/50 div
    const backdrop = document.querySelector('.fixed.inset-0.bg-black\\/50')
    expect(backdrop).toBeInTheDocument()
    fireEvent.click(backdrop!)
    expect(onDecision).toHaveBeenCalledWith('deny')
  })

  it('shows the one-time authorization notice', () => {
    render(<PermissionDialog request={mockRequest} onDecision={vi.fn()} />)

    expect(screen.getByText('此授权仅当次有效，下次访问同一路径需要重新授权。')).toBeInTheDocument()
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
