import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import { WelcomeView } from './WelcomeView'
import type { Settings } from '@/types'

function createSettings(overrides: Partial<Settings> = {}): Settings {
  return {
    model: 'gpt-4',
    agent_id: 'default',
    context_window: 8192,
    working_directory: '',
    language: 'zh-CN',
    base_url: 'https://api.openai.com/v1',
    encrypted_api_key: '',
    model_name: 'gpt-4',
    system_prompt: '你是一个有帮助的 AI 助手。',
    temperature: 0.7,
    close_behavior: 'ask',
    close_remembered: false,
    window_x: 100,
    window_y: 100,
    window_width: 1200,
    window_height: 800,
    sidebar_visible: true,
    sidebar_view: 'conversation',
    ...overrides,
  }
}

describe('WelcomeView', () => {
  const defaultProps = {
    settings: createSettings(),
    inputValue: '',
    onInputChange: vi.fn(),
    onSend: vi.fn(),
  }

  it('renders welcome heading', () => {
    render(<WelcomeView {...defaultProps} />)
    expect(screen.getByText('你好！我是 FlowPartner')).toBeInTheDocument()
  })

  it('renders ChatInput component', () => {
    render(<WelcomeView {...defaultProps} />)
    expect(screen.getByPlaceholderText('输入消息...')).toBeInTheDocument()
  })

  it('displays model name from settings', () => {
    render(<WelcomeView {...defaultProps} settings={createSettings({ model: 'gpt-3.5' })} />)
    expect(screen.getByText(/model: gpt-3.5/)).toBeInTheDocument()
  })

  it('displays agent_id from settings', () => {
    render(<WelcomeView {...defaultProps} settings={createSettings({ agent_id: 'my-agent' })} />)
    expect(screen.getByText(/agent: my-agent/)).toBeInTheDocument()
  })

  it('displays context_window from settings', () => {
    render(<WelcomeView {...defaultProps} settings={createSettings({ context_window: 4096 })} />)
    expect(screen.getByText(/ctx: 4096/)).toBeInTheDocument()
  })

  it('does not display working_directory when empty', () => {
    render(<WelcomeView {...defaultProps} settings={createSettings({ working_directory: '' })} />)
    expect(screen.queryByText(/path:/)).not.toBeInTheDocument()
  })

  it('displays working_directory when set', () => {
    render(<WelcomeView {...defaultProps} settings={createSettings({ working_directory: '/home/user/project' })} />)
    expect(screen.getByText(/path: \/home\/user\/project/)).toBeInTheDocument()
  })

  it('passes input value to ChatInput', () => {
    render(<WelcomeView {...defaultProps} inputValue="typing..." />)
    const input = screen.getByPlaceholderText('输入消息...')
    expect(input).toHaveValue('typing...')
  })

  it('has centered layout', () => {
    const { container } = render(<WelcomeView {...defaultProps} />)
    const wrapper = container.querySelector('.flex-1.flex.flex-col.items-center.justify-center')
    expect(wrapper).toBeInTheDocument()
  })

  it('has max-width constraint on input area', () => {
    const { container } = render(<WelcomeView {...defaultProps} />)
    const inputArea = container.querySelector('.w-full.max-w-2xl')
    expect(inputArea).toBeInTheDocument()
  })

  it('displays info bar with separator', () => {
    render(<WelcomeView {...defaultProps} />)
    // The separator '|' appears between info items (2 separators for 3 info fields)
    const separators = screen.getAllByText('|')
    expect(separators.length).toBe(2)
  })

  it('renders with different model names', () => {
    const models = ['gpt-4', 'gpt-3.5-turbo', 'claude-3', 'llama-2']
    models.forEach(model => {
      const { unmount } = render(
        <WelcomeView {...defaultProps} settings={createSettings({ model })} />
      )
      expect(screen.getByText(new RegExp(`model: ${model}`))).toBeInTheDocument()
      unmount()
    })
  })

  it('renders with large context window', () => {
    render(<WelcomeView {...defaultProps} settings={createSettings({ context_window: 128000 })} />)
    expect(screen.getByText(/ctx: 128000/)).toBeInTheDocument()
  })
})
