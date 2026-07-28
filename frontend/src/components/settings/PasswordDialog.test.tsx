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
    expect(screen.getByText('设置保护密码')).toBeInTheDocument()
  })

  it('does not render when open is false', () => {
    render(<PasswordDialog open={false} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.queryByText('设置保护密码')).not.toBeInTheDocument()
  })

  it('renders password input', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByPlaceholderText('输入密码')).toBeInTheDocument()
  })

  it('renders confirm password input', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByPlaceholderText('确认密码')).toBeInTheDocument()
  })

  it('renders confirm button', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByText('确认')).toBeInTheDocument()
  })

  it('renders cancel button', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.getByText('取消')).toBeInTheDocument()
  })

  it('calls onConfirm with password when form is valid', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('确认'))
    expect(mockOnConfirm).toHaveBeenCalledWith('TestPass123')
  })

  it('shows error when password is too short', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'Ab1' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'Ab1' } })
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('密码需≥8位，包含大小写字母和数字').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when password has no uppercase', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'abcdefgh1' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'abcdefgh1' } })
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('密码需≥8位，包含大小写字母和数字').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when password has no lowercase', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'ABCDEFGH1' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'ABCDEFGH1' } })
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('密码需≥8位，包含大小写字母和数字').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when password has no digit', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'Abcdefgh' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'Abcdefgh' } })
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('密码需≥8位，包含大小写字母和数字').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows error when passwords do not match', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'TestPass456' } })
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('两次输入的密码不一致').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('shows password strength hint when password is weak', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'weak' } })
    expect(screen.getByText('密码需≥8位，包含大小写字母和数字')).toBeInTheDocument()
  })

  it('shows mismatch hint when passwords differ', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'TestPass456' } })
    expect(screen.getAllByText('两次输入的密码不一致').length).toBeGreaterThan(0)
  })

  it('clears inputs and calls onClose when cancel is clicked', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('取消'))
    expect(mockOnClose).toHaveBeenCalled()
    const passwordInput = screen.getByPlaceholderText('输入密码') as HTMLInputElement
    const confirmInput = screen.getByPlaceholderText('确认密码') as HTMLInputElement
    expect(passwordInput.value).toBe('')
    expect(confirmInput.value).toBe('')
  })

  it('clears inputs and error when close icon is clicked', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'weak' } })
    fireEvent.click(screen.getByRole('button', { name: '关闭' }))
    expect(mockOnClose).toHaveBeenCalled()
    const passwordInput = screen.getByPlaceholderText('输入密码') as HTMLInputElement
    expect(passwordInput.value).toBe('')
  })

  it('renders custom title', () => {
    render(
      <PasswordDialog
        open={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        title="修改保护密码"
      />
    )
    expect(screen.getByText('修改保护密码')).toBeInTheDocument()
  })

  it('renders description when provided', () => {
    render(
      <PasswordDialog
        open={true}
        onClose={mockOnClose}
        onConfirm={mockOnConfirm}
        description="请设置一个强密码来保护您的 API Key"
      />
    )
    expect(screen.getByText('请设置一个强密码来保护您的 API Key')).toBeInTheDocument()
  })

  it('does not render description when not provided', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    expect(screen.queryByText('请设置一个强密码来保护您的 API Key')).not.toBeInTheDocument()
  })

  it('clears inputs after successful confirm', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('确认'))

    const passwordInput = screen.getByPlaceholderText('输入密码') as HTMLInputElement
    const confirmInput = screen.getByPlaceholderText('确认密码') as HTMLInputElement
    expect(passwordInput.value).toBe('')
    expect(confirmInput.value).toBe('')
  })

  it('handles empty password submit', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('密码需≥8位，包含大小写字母和数字').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('handles password with only special characters', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: '!@#$%^&*' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: '!@#$%^&*' } })
    fireEvent.click(screen.getByText('确认'))
    expect(screen.getAllByText('密码需≥8位，包含大小写字母和数字').length).toBeGreaterThan(0)
    expect(mockOnConfirm).not.toHaveBeenCalled()
  })

  it('accepts password with special characters and meets requirements', () => {
    render(<PasswordDialog open={true} onClose={mockOnClose} onConfirm={mockOnConfirm} />)
    fireEvent.change(screen.getByPlaceholderText('输入密码'), { target: { value: 'Test@Pass123!' } })
    fireEvent.change(screen.getByPlaceholderText('确认密码'), { target: { value: 'Test@Pass123!' } })
    fireEvent.click(screen.getByText('确认'))
    expect(mockOnConfirm).toHaveBeenCalledWith('Test@Pass123!')
  })
})
