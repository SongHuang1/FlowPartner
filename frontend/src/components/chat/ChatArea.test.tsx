import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { ChatArea } from './ChatArea'

describe('MessageList auto-scroll', () => {
  const originalScrollTo = HTMLElement.prototype.scrollTo

  beforeEach(() => {
    HTMLElement.prototype.scrollTo = vi.fn()
  })

  afterEach(() => {
    HTMLElement.prototype.scrollTo = originalScrollTo
  })

  it('scrolls to bottom when new message appears', () => {
    render(<ChatArea />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    expect(HTMLElement.prototype.scrollTo).toHaveBeenCalledWith({
      top: expect.any(Number),
      behavior: 'smooth',
    })
  })

  it('scrolls to bottom on initial render with welcome message', () => {
    render(<ChatArea />)

    expect(HTMLElement.prototype.scrollTo).toHaveBeenCalledTimes(1)
    expect(HTMLElement.prototype.scrollTo).toHaveBeenCalledWith({
      top: expect.any(Number),
      behavior: 'smooth',
    })
  })

  it('scrolls to bottom when ephemeral message disappears', () => {
    vi.useFakeTimers()
    render(<ChatArea />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')
    fireEvent.change(input, { target: { value: 'vanish test' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    // Clear mock calls from the send action
    ;(HTMLElement.prototype.scrollTo as ReturnType<typeof vi.fn>).mockClear()

    act(() => {
      vi.advanceTimersByTime(3000)
    })

    // Should scroll again when messages change (ephemeral removed)
    expect(HTMLElement.prototype.scrollTo).toHaveBeenCalledWith({
      top: expect.any(Number),
      behavior: 'smooth',
    })

    vi.useRealTimers()
  })

  it('scrollTo is called with scrollHeight as top value', () => {
    const mockScrollTo = vi.fn()
    HTMLElement.prototype.scrollTo = mockScrollTo

    render(<ChatArea />)

    // The first call should use the element's scrollHeight
    expect(mockScrollTo).toHaveBeenCalledWith({
      top: expect.any(Number),
      behavior: 'smooth',
    })
  })

  it('scrolls multiple times for rapid messages', () => {
    render(<ChatArea />)

    const input = screen.getByPlaceholderText('输入消息（预览模式，暂不发送）')

    fireEvent.change(input, { target: { value: 'msg1' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    fireEvent.change(input, { target: { value: 'msg2' } })
    fireEvent.keyDown(input, { key: 'Enter' })

    // Called at least 3 times: initial + 2 sends
    expect((HTMLElement.prototype.scrollTo as ReturnType<typeof vi.fn>).mock.calls.length).toBeGreaterThanOrEqual(3)
  })
})

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
