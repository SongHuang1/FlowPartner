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
    expect(textarea.value).toBe('你是一个有帮助的 AI 助手。')
  })

  it('displays current temperature value', () => {
    render(<AgentSettings />)
    expect(screen.getByText(/0\.7/)).toBeInTheDocument()
  })

  it('calls updateSettings when system prompt changes', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词')
    fireEvent.change(textarea, { target: { value: '新的系统提示词' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: '新的系统提示词' })
  })

  it('calls updateSettings when temperature changes', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '1.5' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ temperature: 1.5 })
  })

  it('renders temperature range labels', () => {
    render(<AgentSettings />)
    expect(screen.getByText('0.0 (精确)')).toBeInTheDocument()
    expect(screen.getByText('2.0 (创意)')).toBeInTheDocument()
  })

  it('renders section title', () => {
    render(<AgentSettings />)
    expect(screen.getByText('Agent 配置')).toBeInTheDocument()
  })

  it('renders textarea with correct placeholder', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('系统提示词') as HTMLTextAreaElement
    expect(textarea.placeholder).toBe('你是一个有帮助的 AI 助手。')
  })

  it('renders slider with correct min/max/step', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider') as HTMLInputElement
    expect(slider.min).toBe('0')
    expect(slider.max).toBe('2')
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

  it('handles temperature boundary 2.0', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '2' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ temperature: 2 })
  })
})
