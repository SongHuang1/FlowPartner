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
    expect(screen.getByText('Unlock API Key')).toBeInTheDocument()
  })

  it('does not render when open is false', () => {
    render(<UnlockDialog open={false} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.queryByText('Unlock API Key')).not.toBeInTheDocument()
  })

  it('renders password input', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByPlaceholderText('Enter protection password')).toBeInTheDocument()
  })

  it('renders unlock button', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByText('Unlock')).toBeInTheDocument()
  })

  it('renders cancel button', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.getByText('Cancel')).toBeInTheDocument()
  })

  it('calls onUnlock with password when unlock button clicked', async () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('Enter protection password')
    fireEvent.change(passwordInput, { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('Unlock'))

    await waitFor(() => {
      expect(mockOnUnlock).toHaveBeenCalledWith('TestPass123')
    })
  })

  it('calls onClose when cancel button clicked', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    fireEvent.click(screen.getByText('Cancel'))
    expect(mockOnClose).toHaveBeenCalled()
  })

  it('calls onClose when close icon clicked', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(mockOnClose).toHaveBeenCalled()
  })

  it('shows error when unlock fails', async () => {
    mockOnUnlock.mockRejectedValue(new Error('Wrong password'))
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('Enter protection password')
    fireEvent.change(passwordInput, { target: { value: 'WrongPass123' } })
    fireEvent.click(screen.getByText('Unlock'))

    await waitFor(() => {
      expect(screen.getByText('Wrong password')).toBeInTheDocument()
    })
  })

  it('clears password after failed unlock', async () => {
    mockOnUnlock.mockRejectedValue(new Error('Wrong password'))
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('Enter protection password') as HTMLInputElement
    fireEvent.change(passwordInput, { target: { value: 'WrongPass123' } })
    fireEvent.click(screen.getByText('Unlock'))

    await waitFor(() => {
      expect(passwordInput.value).toBe('')
    })
  })

  it('shows error when password is empty', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    fireEvent.click(screen.getByText('Unlock'))
    expect(screen.getByText('Please enter your password')).toBeInTheDocument()
  })

  it('shows failed attempts warning when failedAttempts > 0', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={3} />)
    expect(screen.getByText(/3 failed attempts/)).toBeInTheDocument()
  })

  it('does not show failed attempts warning when failedAttempts is 0', () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    expect(screen.queryByText(/failed attempt/)).not.toBeInTheDocument()
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
    expect(screen.getByText(/Account locked/)).toBeInTheDocument()
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
    const passwordInput = screen.getByPlaceholderText('Enter protection password') as HTMLInputElement
    const unlockButton = screen.getByText('Unlock')
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
    expect(screen.queryByText(/Account locked/)).not.toBeInTheDocument()
  })

  it('shows loading state during unlock', async () => {
    let resolveUnlock: (value: unknown) => void
    mockOnUnlock.mockImplementation(() => new Promise((resolve) => { resolveUnlock = resolve }))

    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('Enter protection password')
    fireEvent.change(passwordInput, { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('Unlock'))

    expect(screen.getByText('Unlocking...')).toBeInTheDocument()

    await waitFor(() => {
      resolveUnlock!(undefined)
    })
  })

  it('clears password and closes on successful unlock', async () => {
    render(<UnlockDialog open={true} onClose={mockOnClose} onUnlock={mockOnUnlock} failedAttempts={0} />)
    const passwordInput = screen.getByPlaceholderText('Enter protection password') as HTMLInputElement
    fireEvent.change(passwordInput, { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('Unlock'))

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
    expect(screen.queryByText(/failed attempt/)).not.toBeInTheDocument()
  })
})
