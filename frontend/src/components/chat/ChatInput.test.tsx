import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ChatInput } from './ChatArea'

describe('ChatInput', () => {
  it('renders input with correct placeholder', () => {
    render(<ChatInput onSend={() => {}} />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    expect(input).toBeInTheDocument()
  })

  it('renders send button', () => {
    render(<ChatInput onSend={() => {}} />)
    expect(screen.getByRole('button', { name: '发送' })).toBeInTheDocument()
  })

  it('send button is disabled when input is empty', () => {
    render(<ChatInput onSend={() => {}} />)
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })

  it('send button is enabled when input has text', () => {
    render(<ChatInput onSend={() => {}} />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'hello' } })

    const button = screen.getByRole('button', { name: '发送' })
    expect(button).not.toBeDisabled()
  })

  it('send button is disabled when input has only whitespace', () => {
    render(<ChatInput onSend={() => {}} />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: '   ' } })

    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })

  it('calls onSend with trimmed text when clicking send button', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: '  hello world  ' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(onSend).toHaveBeenCalledWith('hello world')
    expect(onSend).toHaveBeenCalledTimes(1)
  })

  it('calls onSend when pressing Enter', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'enter test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onSend).toHaveBeenCalledWith('enter test')
  })

  it('does not call onSend when pressing Shift+Enter', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'shift enter' } })
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })

    expect(onSend).not.toHaveBeenCalled()
  })

  it('clears input after sending', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'clear me' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(input).toHaveValue('')
  })

  it('does not call onSend when input is empty and Enter is pressed', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onSend).not.toHaveBeenCalled()
  })

  it('does not call onSend when input is whitespace-only and Enter is pressed', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: '   ' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onSend).not.toHaveBeenCalled()
  })

  it('focuses input after sending', () => {
    const onSend = vi.fn()
    render(<ChatInput onSend={onSend} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'focus test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(document.activeElement).toBe(input)
  })

  it('input value updates correctly on change', () => {
    render(<ChatInput onSend={() => {}} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'typing...' } })

    expect(input).toHaveValue('typing...')
  })

  it('button disabled state updates dynamically', () => {
    render(<ChatInput onSend={() => {}} />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    const button = screen.getByRole('button', { name: '发送' })

    // Initially disabled
    expect(button).toBeDisabled()

    // Type something → enabled
    fireEvent.change(input, { target: { value: 'a' } })
    expect(button).not.toBeDisabled()

    // Clear input → disabled again
    fireEvent.change(input, { target: { value: '' } })
    expect(button).toBeDisabled()
  })
})
