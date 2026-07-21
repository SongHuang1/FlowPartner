import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { ChatArea } from './ChatArea'

describe('ChatArea', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  it('renders welcome message on mount', () => {
    render(<ChatArea />)
    expect(screen.getByText('你好！我是你的 AI 助手 FlowPartner，有什么需要帮忙的吗？')).toBeInTheDocument()
  })

  it('send button is disabled when input is empty', () => {
    render(<ChatArea />)
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).toBeDisabled()
  })

  it('send button is enabled when input has text', () => {
    render(<ChatArea />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'hello' } })
    const button = screen.getByRole('button', { name: '发送' })
    expect(button).not.toBeDisabled()
  })

  it('shows ephemeral bubble after clicking send', () => {
    render(<ChatArea />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'test message' } })
    const button = screen.getByRole('button', { name: '发送' })
    fireEvent.click(button)

    expect(screen.getByText('test message')).toBeInTheDocument()
    expect(input).toHaveValue('')
  })

  it('shows ephemeral bubble after pressing Enter', () => {
    render(<ChatArea />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'enter test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(screen.getByText('enter test')).toBeInTheDocument()
  })

  it('ephemeral bubble disappears after 3 seconds', () => {
    render(<ChatArea />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'vanish' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(screen.getByText('vanish')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(3000)
    })

    expect(screen.queryByText('vanish')).not.toBeInTheDocument()
    expect(screen.getByText('你好！我是你的 AI 助手 FlowPartner，有什么需要帮忙的吗？')).toBeInTheDocument()
  })

  it('rapid consecutive sends reset timer', () => {
    render(<ChatArea />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')

    fireEvent.change(input, { target: { value: 'first' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.getByText('first')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(2000)
    })

    fireEvent.change(input, { target: { value: 'second' } })
    fireEvent.keyDown(input, { key: 'Enter' })
    expect(screen.queryByText('first')).not.toBeInTheDocument()
    expect(screen.getByText('second')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(2000)
    })
    expect(screen.getByText('second')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(1000)
    })
    expect(screen.queryByText('second')).not.toBeInTheDocument()
  })

  it('cleans up timeout on unmount', () => {
    const { unmount } = render(<ChatArea />)
    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'unmount test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    act(() => {
      unmount()
    })

    // Should not throw and timer should be cleaned up
    act(() => {
      vi.advanceTimersByTime(4000)
    })
  })
})
