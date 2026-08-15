import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { AgentSettings } from '@/components/settings/AgentSettings'

const mockUpdateSettings = vi.fn()

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({
    settings: {
      model: 'gpt-4',
      agent_id: 'default',
      context_window: 8192,
      working_directory: '',
      language: 'zh-CN',
      base_url: 'https://api.openai.com/v1',
      encrypted_api_key: '',
      model_name: 'gpt-4',
      system_prompt: '你是一个乐于助人的 AI 助手。',
      temperature: 0.7,
      close_behavior: 'ask',
      close_remembered: false,
      window_x: 100,
      window_y: 100,
      window_width: 1200,
      window_height: 800,
      sidebar_visible: true,
      sidebar_view: 'conversation',
    },
    updateSettings: mockUpdateSettings,
  }),
}))

describe('AgentSettings', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('renders system prompt textarea', () => {
    render(<AgentSettings />)
    expect(screen.getByLabelText('系统提示词')).toBeInTheDocument()
  })

  it('renders temperature slider', () => {
    render(<AgentSettings />)
    expect(screen.getByText(/温度/)).toBeInTheDocument()
  })

  it('displays current system prompt value', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词') as HTMLTextAreaElement
    expect(textarea.value).toBe('你是一个乐于助人的 AI 助手。')
  })

  it('displays current temperature value', () => {
    render(<AgentSettings />)
    expect(screen.getByText(/0\.7/)).toBeInTheDocument()
  })

  it('calls updateSettings when system prompt changes', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词')
    fireEvent.change(textarea, { target: { value: 'new system prompt' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: 'new system prompt' })
  })

  it('calls updateSettings when temperature changes', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '0.8' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ temperature: 0.8 })
  })

it('renders temperature range labels', () => {
    render(<AgentSettings />)
    expect(screen.getByText('0.0 精确')).toBeInTheDocument()
    expect(screen.getByText('0.5 平衡')).toBeInTheDocument()
    expect(screen.getByText('1.0 创意')).toBeInTheDocument()
  })

  it('renders 基础设置 and 对话参数 section titles', () => {
    render(<AgentSettings />)
    expect(screen.getByText('基础设置')).toBeInTheDocument()
    expect(screen.getByText('对话参数')).toBeInTheDocument()
  })

  it('renders textarea with correct placeholder', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词') as HTMLTextAreaElement
    expect(textarea.placeholder).toBe('你是一个乐于助人的 AI 助手。')
  })

  it('renders slider with correct min/max/step', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider') as HTMLInputElement
    expect(slider.min).toBe('0')
    expect(slider.max).toBe('1')
    expect(slider.step).toBe('0.1')
  })

  it('handles empty system prompt', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词')
    fireEvent.change(textarea, { target: { value: '' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: '' })
  })

  it('handles unicode system prompt', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词')
    fireEvent.change(textarea, { target: { value: '日本語テスト 🎌' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: '日本語テスト 🎌' })
  })

  it('handles long system prompt', () => {
    render(<AgentSettings />)
    const longPrompt = '你是一个专家。'.repeat(100)
    const textarea = screen.getByLabelText('系统提示词')
    fireEvent.change(textarea, { target: { value: longPrompt } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: longPrompt })
  })

  it('handles temperature boundary 0.0', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '0' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ temperature: 0 })
  })

  it('handles temperature boundary 1.0', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '1' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ temperature: 1 })
  })

  it('renders agent ID input with current value', () => {
    render(<AgentSettings />)
    const input = screen.getByLabelText('智能体 ID') as HTMLInputElement
    expect(input.value).toBe('default')
  })

  it('calls updateSettings when agent ID changes', () => {
    render(<AgentSettings />)
    const input = screen.getByLabelText('智能体 ID')
    fireEvent.change(input, { target: { value: 'my-agent' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ agent_id: 'my-agent' })
  })

  it('renders context window input with current value', () => {
    render(<AgentSettings />)
    const input = screen.getByLabelText('上下文窗口') as HTMLInputElement
    expect(input.value).toBe('8192')
  })

  it('parses context window on blur and updates settings', () => {
    render(<AgentSettings />)
    const input = screen.getByLabelText('上下文窗口')
    fireEvent.change(input, { target: { value: '16384' } })
    fireEvent.blur(input)
    expect(mockUpdateSettings).toHaveBeenCalledWith({ context_window: 16384 })
  })

  it('resets context window to 1 on invalid blur value', () => {
    render(<AgentSettings />)
    const input = screen.getByLabelText('上下文窗口')
    fireEvent.change(input, { target: { value: 'abc' } })
    fireEvent.blur(input)
    expect(mockUpdateSettings).toHaveBeenCalledWith({ context_window: 1 })
    expect((screen.getByLabelText('上下文窗口') as HTMLInputElement).value).toBe('1')
  })

  it('resets context window to 1 on blur value below minimum', () => {
    render(<AgentSettings />)
    const input = screen.getByLabelText('上下文窗口')
    fireEvent.change(input, { target: { value: '0' } })
    fireEvent.blur(input)
    expect(mockUpdateSettings).toHaveBeenCalledWith({ context_window: 1 })
  })
})
