import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { AgentsManager } from '@/components/settings/AgentsManager'
import { SettingsProvider } from '@/hooks/useSettings'

const mockListAgents = vi.fn()
const mockGetAgent = vi.fn()
const mockCreateAgent = vi.fn()
const mockUpdateAgent = vi.fn()
const mockDeleteAgent = vi.fn()

const mockGetSettings = vi.fn()
const mockSaveSettings = vi.fn()

vi.mock('@/lib/api', () => ({
  listAgents: () => mockListAgents(),
  getAgent: (id: string) => mockGetAgent(id),
  createAgent: (input: unknown) => mockCreateAgent(input),
  updateAgent: (id: string, input: unknown) => mockUpdateAgent(id, input),
  deleteAgent: (id: string) => mockDeleteAgent(id),
  getSettings: () => mockGetSettings(),
  saveSettings: (input: unknown) => mockSaveSettings(input),
}))

const mainAgent = { id: 'main', name: '主智能体', description: '默认执行者' }
const userAgent = { id: 'agent-1', name: '翻译官', description: '负责翻译' }
const userAgentDetail = {
  ...userAgent,
  system_prompt: '你是专业翻译。',
  created_at: 1,
  updated_at: 1,
}

describe('AgentsManager', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockListAgents.mockResolvedValue([mainAgent, userAgent])
    mockGetAgent.mockResolvedValue(userAgentDetail)
    mockCreateAgent.mockResolvedValue(userAgentDetail)
    mockUpdateAgent.mockResolvedValue(userAgentDetail)
    mockDeleteAgent.mockResolvedValue(undefined)
    mockGetSettings.mockResolvedValue({
      model: 'gpt-4',
      agent_id: 'main',
      context_window: 8192,
      working_directory: '',
      trash_dir: '',
      language: 'zh-CN',
      base_url: 'https://api.openai.com/v1',
      encrypted_api_key: '',
      model_name: 'gpt-4',
      system_prompt: '',
      temperature: 0.7,
      close_behavior: 'ask',
      close_remembered: false,
      window_x: 0,
      window_y: 0,
      window_width: 1200,
      window_height: 800,
      sidebar_visible: true,
      sidebar_view: 'conversation',
      snapshot_dir: '',
      snapshot_enabled: false,
      snapshot_include_secrets: false,
    })
    mockSaveSettings.mockResolvedValue(undefined)
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  it('renders agent list with builtin badge', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    expect(await screen.findByText('翻译官')).toBeInTheDocument()
    expect(screen.getByText('主智能体参数')).toBeInTheDocument()
    expect(screen.getByText('负责翻译')).toBeInTheDocument()
  })

  it('creates a new agent via form', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: /新建智能体/ }))
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '写手' } })
    fireEvent.change(screen.getByLabelText('对外描述'), { target: { value: '负责写作' } })
    fireEvent.change(screen.getAllByLabelText('系统提示词')[1], { target: { value: '你是写手。' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(mockCreateAgent).toHaveBeenCalledWith({
        name: '写手',
        description: '负责写作',
        system_prompt: '你是写手。',
      })
    })
    expect(mockListAgents).toHaveBeenCalledTimes(2)
  })

  it('requires all fields before saving', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: /新建智能体/ }))
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(screen.getByText(/均为必填项/)).toBeInTheDocument()
    })
    expect(mockCreateAgent).not.toHaveBeenCalled()
  })

  it('loads system_prompt only when editing', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    expect(screen.queryByText('你是专业翻译。')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '编辑 翻译官' }))
    await waitFor(() => {
      expect(screen.getAllByLabelText('系统提示词')[1]).toHaveValue('你是专业翻译。')
    })
    expect(mockGetAgent).toHaveBeenCalledWith('agent-1')
  })

  it('updates an existing agent', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: '编辑 翻译官' }))
    await waitFor(() => {
      expect(screen.getAllByLabelText('系统提示词')[1]).toHaveValue('你是专业翻译。')
    })

    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '翻译官V2' } })
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(mockUpdateAgent).toHaveBeenCalledWith('agent-1', {
        name: '翻译官V2',
        description: '负责翻译',
        system_prompt: '你是专业翻译。',
      })
    })
  })

  it('deletes an agent after confirm', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: '删除 翻译官' }))

    await waitFor(() => {
      expect(mockDeleteAgent).toHaveBeenCalledWith('agent-1')
    })
  })

  it('does not offer delete for builtin main agent', async () => {
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    expect(screen.queryByRole('button', { name: '删除 主智能体' })).not.toBeInTheDocument()
  })

  it('shows error when delete fails', async () => {
    mockDeleteAgent.mockRejectedValue(new Error('删除失败：内置智能体不可删除'))
    render(<SettingsProvider><AgentsManager /></SettingsProvider>)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: '删除 翻译官' }))

    await waitFor(() => {
      expect(screen.getByText(/删除失败/)).toBeInTheDocument()
    })
  })
})
