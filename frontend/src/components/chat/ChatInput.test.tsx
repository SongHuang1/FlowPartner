import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { ChatInput } from './ChatArea'

function renderChatInput(onSend: () => void, initialValue = '') {
  function Wrapper() {
    const [value, setValue] = useState(initialValue)
    return (
      <ChatInput
        value={value}
        onChange={setValue}
        onSend={() => {
          onSend()
          setValue('')
        }}
      />
    )
  }
  return render(<Wrapper />)
}

import { useState } from 'react'

describe('ChatInput', () => {
  it('renders input with correct placeholder', () => {
    renderChatInput(() => {})
    const input = screen.getByPlaceholderText('输入消息...')
    expect(input).toBeInTheDocument()
  })

  it('renders send button', () => {
    renderChatInput(() => {})
    expect(screen.getByRole('button', { name: '发送' })).toBeInTheDocument()
  })

  it('send button is disabled when input is empty', () => {
    renderChatInput(() => {})
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })

  it('send button is enabled when input has text', () => {
    renderChatInput(() => {})
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'hello' } })

    const button = screen.getByRole('button', { name: '发送' })
    expect(button).not.toBeDisabled()
  })

  it('send button is disabled when input has only whitespace', () => {
    renderChatInput(() => {})
    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '   ' } })

    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })

  it('calls onSend when clicking send button', () => {
    const onSend = vi.fn()
    renderChatInput(onSend)

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'hello' } })
    fireEvent.click(screen.getByRole('button', { name: '发送' }))

    expect(onSend).toHaveBeenCalledTimes(1)
  })

  it('calls onSend when pressing Enter', () => {
    const onSend = vi.fn()
    renderChatInput(onSend)

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'enter test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onSend).toHaveBeenCalledTimes(1)
  })

  it('does not call onSend when pressing Shift+Enter', () => {
    const onSend = vi.fn()
    renderChatInput(onSend)

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'shift enter' } })
    fireEvent.keyDown(input, { key: 'Enter', shiftKey: true })

    expect(onSend).not.toHaveBeenCalled()
  })

  it('does not call onSend when input is empty and Enter is pressed', () => {
    const onSend = vi.fn()
    renderChatInput(onSend)

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onSend).not.toHaveBeenCalled()
  })

  it('does not call onSend when input is whitespace-only and Enter is pressed', () => {
    const onSend = vi.fn()
    renderChatInput(onSend)

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: '   ' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(onSend).not.toHaveBeenCalled()
  })

  it('focuses input after sending', () => {
    const onSend = vi.fn()
    renderChatInput(onSend)

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'focus test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(document.activeElement).toBe(input)
  })

  it('input value updates correctly on change', () => {
    renderChatInput(() => {})

    const input = screen.getByPlaceholderText('输入消息...')
    fireEvent.change(input, { target: { value: 'typing...' } })

    expect(input).toHaveValue('typing...')
  })

  it('button disabled state updates dynamically', () => {
    renderChatInput(() => {})

    const input = screen.getByPlaceholderText('输入消息...')
    const button = screen.getByRole('button', { name: '发送' })

    expect(button).toBeDisabled()

    fireEvent.change(input, { target: { value: 'a' } })
    expect(button).not.toBeDisabled()

    fireEvent.change(input, { target: { value: '' } })
    expect(button).toBeDisabled()
  })
})
