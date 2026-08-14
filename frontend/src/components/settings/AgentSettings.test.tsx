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
      system_prompt: 'You are a helpful AI assistant.',
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
    expect(screen.getByLabelText('System prompt')).toBeInTheDocument()
  })

  it('renders temperature slider', () => {
    render(<AgentSettings />)
    expect(screen.getByText(/Temperature/)).toBeInTheDocument()
  })

  it('displays current system prompt value', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('System prompt') as HTMLTextAreaElement
    expect(textarea.value).toBe('You are a helpful AI assistant.')
  })

  it('displays current temperature value', () => {
    render(<AgentSettings />)
    expect(screen.getByText(/0\.7/)).toBeInTheDocument()
  })

  it('calls updateSettings when system prompt changes', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('System prompt')
    fireEvent.change(textarea, { target: { value: 'new system prompt' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: 'new system prompt' })
  })

  it('calls updateSettings when temperature changes', () => {
    render(<AgentSettings />)
    const slider = screen.getByRole('slider')
    fireEvent.change(slider, { target: { value: '1.5' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ temperature: 1.5 })
  })

  it('renders temperature range labels', () => {
    render(<AgentSettings />)
    expect(screen.getByText('0.0 (Precise)')).toBeInTheDocument()
    expect(screen.getByText('2.0 (Creative)')).toBeInTheDocument()
  })

  it('renders section title', () => {
    render(<AgentSettings />)
    expect(screen.getByText('Agent Settings')).toBeInTheDocument()
  })

  it('renders textarea with correct placeholder', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('System prompt') as HTMLTextAreaElement
    expect(textarea.placeholder).toBe('You are a helpful AI assistant.')
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
    const textarea = screen.getByLabelText('System prompt')
    fireEvent.change(textarea, { target: { value: '' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: '' })
  })

  it('handles unicode system prompt', () => {
    render(<AgentSettings />)
    const textarea = screen.getByLabelText('System prompt')
    fireEvent.change(textarea, { target: { value: '日本語テスト 🎌' } })
    expect(mockUpdateSettings).toHaveBeenCalledWith({ system_prompt: '日本語テスト 🎌' })
  })

  it('handles long system prompt', () => {
    render(<AgentSettings />)
    const longPrompt = '你是一个专家。'.repeat(100)
    const textarea = screen.getByLabelText('System prompt')
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
