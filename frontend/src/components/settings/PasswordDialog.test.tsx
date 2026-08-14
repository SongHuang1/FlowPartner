import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { PasswordDialog } from '@/components/settings/PasswordDialog'

const mockOnClose = vi.fn()
const mockOnConfirm = vi.fn()

describe('PasswordDialog', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders when open is true', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByText('Set protection password')).toBeInTheDocument()
  })

  it('does not render when open is false', () => {
    render(<PasswordDialog open={false} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.queryByText('Set protection password')).not.toBeInTheDocument()
  })

  it('renders password input', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByPlaceholderText('Enter password')).toBeInTheDocument()
  })

  it('renders confirm password input', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByPlaceholderText('Confirm password')).toBeInTheDocument()
  })

  it('renders confirm button', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByText('Confirm')).toBeInTheDocument()
  })

  it('renders cancel button', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByText('Cancel')).toBeInTheDocument()
  })

  it('calls onConfirm with password when form is valid', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(mockOnConfirm).toHaveBeenCalledWith('TestPass123')
  })

  it('shows error when password is too short', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'Ab1' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'Ab1' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('Password must be at least 8 characters with uppercase, lowercase and numbers').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when password has no uppercase', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'abcdefgh1' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'abcdefgh1' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('Password must be at least 8 characters with uppercase, lowercase and numbers').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when password has no lowercase', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'ABCDEFGH1' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'ABCDEFGH1' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('Password must be at least 8 characters with uppercase, lowercase and numbers').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when password has no digit', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'Abcdefgh' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'Abcdefgh' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('Password must be at least 8 characters with uppercase, lowercase and numbers').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when passwords do not match', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'TestPass456' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('The two passwords do not match').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows password strength hint when password is weak', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'weak' } })
    expect(screen.getByText('Password must be at least 8 characters with uppercase, lowercase and numbers')).toBeInTheDocument()
  })

  it('shows mismatch hint when passwords differ', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'TestPass456' } })
    expect(screen.getAllByText('The two passwords do not match').length).toBeGreaterThan(0)
  })

  it('clears inputs and calls onClose when cancel is clicked', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('Cancel'))
    expect(mockOnClose).toHaveBeenCalled()
    const passwordInput = screen.getByPlaceholderText('Enter password') as HTMLInputElement
    const confirmInput = screen.getByPlaceholderText('Confirm password') as HTMLInputElement
    expect(passwordInput.value).toBe('')
    expect(confirmInput.value).toBe('')
  })

  it('clears inputs and error when close icon is clicked', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'weak' } })
    fireEvent.click(screen.getByRole('button', { name: 'Close' }))
    expect(mockOnClose).toHaveBeenCalled()
    const passwordInput = screen.getByPlaceholderText('Enter password') as HTMLInputElement
    expect(passwordInput.value).toBe('')
  })

  it('renders custom title', () => {
    render(
      <PasswordDialog
        open={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        title="Change protection password"
      />
    )
    expect(screen.getByText('Change protection password')).toBeInTheDocument()
  })

  it('renders description when provided', () => {
    render(
      <PasswordDialog
        open={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        description="Please set a strong password to protect your API Key"
      />
    )
    expect(screen.getByText('Please set a strong password to protect your API Key')).toBeInTheDocument()
  })

  it('does not render description when not provided', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.queryByText('Please set a strong password to protect your API Key')).not.toBeInTheDocument()
  })

  it('clears inputs after successful confirm', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('Confirm'))

    const passwordInput = screen.getByPlaceholderText('Enter password') as HTMLInputElement
    const confirmInput = screen.getByPlaceholderText('Confirm password') as HTMLInputElement
    expect(passwordInput.value).toBe('')
    expect(confirmInput.value).toBe('')
  })

  it('handles empty password submit', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('Password must be at least 8 characters with uppercase, lowercase and numbers').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('handles password with only special characters', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: '!@#$%^&*' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: '!@#$%^&*' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(screen.getAllByText('Password must be at least 8 characters with uppercase, lowercase and numbers').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('accepts password with special characters and meets requirements', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('Enter password'), { target: { value: 'Test@Pass123!' } })
    fireEvent.change(screen.getByPlaceholderText('Confirm password'), { target: { value: 'Test@Pass123!' } })
    fireEvent.click(screen.getByText('Confirm'))
    expect(mockOnConfirm).toHaveBeenCalledWith('Test@Pass123!')
  })
})
