import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { UnlockDialog } from '@/components/settings/UnlockDialog'

const mockOnClose = vi.fn()
const mockOnUnlock = vi.fn()

describe('UnlockDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockOnUnlock.mockResolvedValue(undefined)
  })

  it('renders when open is true', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByText('解锁 API Key')).toBeInTheDocument()
  })

  it('does not render when open is false', () => {
    render(<UnlockDialog open={false} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.queryByText('解锁 API Key')).not.toBeInTheDocument()
  })

  it('renders password input', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByPlaceholderText('输入保护密码')).toBeInTheDocument()
  })

  it('renders unlock button', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByText('解锁')).toBeInTheDocument()
  })

  it('renders cancel button', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByText('取消')).toBeInTheDocument()
  })

  it('calls onUnlock with password when unlock button clicked', async () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('输入保护密码')
    fireEvent.change(passwordInput, { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    await waitFor(() => {
      expect(mockOnUnlock).toHaveBeenCalledWith('TestPass123')
    })
  })

  it('calls onClose when cancel button clicked', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    fireEvent.click(screen.getByText('取消'))
    expect(mockOnClose).toHaveBeenCalled()
  })

  it('calls onClose when close icon clicked', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    fireEvent.click(screen.getByRole('button', { name: '关闭' }))
    expect(mockOnClose).toHaveBeenCalled()
  })

  it('shows error when unlock fails', async () => {
    mockOnUnlock.mockRejectedValue(new Error('密码错误'))
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('输入保护密码')
    fireEvent.change(passwordInput, { target: { value: 'WrongPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    await waitFor(() => {
      expect(screen.getByText('密码错误')).toBeInTheDocument()
    })
  })

  it('clears password after failed unlock', async () => {
    mockOnUnlock.mockRejectedValue(new Error('密码错误'))
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('输入保护密码') as HTMLInputElement
    fireEvent.change(passwordInput, { target: { value: 'WrongPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    await waitFor(() => {
      expect(passwordInput.value).toBe('')
    })
  })

  it('shows error when password is empty', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    fireEvent.click(screen.getByText('解锁'))
    expect(screen.getByText('请输入密码')).toBeInTheDocument()
  })

  it('shows failed attempts warning when failedAttempts > 0', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={3} />)
    expect(screen.getByText(/已失败 3 次/)).toBeInTheDocument()
  })

  it('does not show failed attempts warning when failedAttempts is 0', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.queryByText(/已失败/)).not.toBeInTheDocument()
  })

  it('shows locked message when lockedUntil is in the future', () => {
    const futureTime = new Date(Date.now() + 30000).toISOString()
    render(
      <UnlockDialog
        open={true}
        onClose={mockOnClose}
        onUnlock={mockOnUnlock}
        failedAttempts={5}
        lockedUntil={futureTime}
      />
    )
    expect(screen.getByText(/账户已锁定/)).toBeInTheDocument()
  })

  it('disables input and button when locked', () => {
    const futureTime = new Date(Date.now() + 30000).toISOString()
    render(
      <UnlockDialog
        open={true}
        onClose={mockOnClose}
        onUnlock={mockOnUnlock}
        failedAttempts={5}
        lockedUntil={futureTime}
      />
    )
    const passwordInput = screen.getByPlaceholderText('输入保护密码') as HTMLInputElement
    const unlockButton = screen.getByText('解锁')
    expect(passwordInput.disabled).toBe(true)
    expect(unlockButton).toBeDisabled()
  })

  it('does not show locked message when lockedUntil is in the past', () => {
    const pastTime = new Date(Date.now() - 30000).toISOString()
    render(
      <UnlockDialog
        open={true}
        onClose={mockOnClose}
        onUnlock={mockOnUnlock}
        failedAttempts={0}
        lockedUntil={pastTime}
      />
    )
    expect(screen.queryByText(/账户已锁定/)).not.toBeInTheDocument()
  })

  it('shows loading state during unlock', async () => {
    let resolveUnlock: (value: unknown) => void
    mockOnUnlock.mockImplementation(() => new Promise((resolve) => { resolveUnlock = resolve }))

    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('输入保护密码')
    fireEvent.change(passwordInput, { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    expect(screen.getByText('解锁中...')).toBeInTheDocument()

    await waitFor(() => {
      resolveUnlock!(undefined)
    })
  })

  it('clears password and closes on successful unlock', async () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('输入保护密码') as HTMLInputElement
    fireEvent.change(passwordInput, { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    await waitFor(() => {
      expect(mockOnClose).toHaveBeenCalled()
    })
    expect(passwordInput.value).toBe('')
  })

  it('does not show failed attempts warning when locked', () => {
    const futureTime = new Date(Date.now() + 30000).toISOString()
    render(
      <UnlockDialog
        open={true}
        onClose={mockOnClose}
        onUnlock={mockOnUnlock}
        failedAttempts={3}
        lockedUntil={futureTime}
      />
    )
    // When locked, the failed attempts warning should not show
    expect(screen.queryByText(/已失败/)).not.toBeInTheDocument()
  })
})
