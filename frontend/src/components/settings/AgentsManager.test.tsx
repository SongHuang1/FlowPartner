import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { AgentsManager } from '@/components/settings/AgentsManager'

const mockListAgents = vi.fn()
const mockGetAgent = vi.fn()
const mockCreateAgent = vi.fn()
const mockUpdateAgent = vi.fn()
const mockDeleteAgent = vi.fn()

vi.mock('@/lib/api', () => ({
  listAgents: () => mockListAgents(),
  getAgent: (id: string) => mockGetAgent(id),
  createAgent: (input: unknown) => mockCreateAgent(input),
  updateAgent: (id: string, input: unknown) => mockUpdateAgent(id, input),
  deleteAgent: (id: string) => mockDeleteAgent(id),
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
    vi.spyOn(window, 'confirm').mockReturnValue(true)
  })

  it('renders agent list with builtin badge', async () => {
    render(<AgentsManager />)
    expect(await screen.findByText('翻译官')).toBeInTheDocument()
    expect(screen.getByText('主智能体')).toBeInTheDocument()
    expect(screen.getByText('内置')).toBeInTheDocument()
    expect(screen.getByText('负责翻译')).toBeInTheDocument()
  })

  it('creates a new agent via form', async () => {
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: /新建智能体/ }))
    fireEvent.change(screen.getByLabelText('名称'), { target: { value: '写手' } })
    fireEvent.change(screen.getByLabelText('对外描述'), { target: { value: '负责写作' } })
    fireEvent.change(screen.getByLabelText('系统提示词（仅编辑时可见）'), { target: { value: '你是写手。' } })
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
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: /新建智能体/ }))
    fireEvent.click(screen.getByRole('button', { name: '保存' }))

    await waitFor(() => {
      expect(screen.getByText(/均为必填项/)).toBeInTheDocument()
    })
    expect(mockCreateAgent).not.toHaveBeenCalled()
  })

  it('loads system_prompt only when editing', async () => {
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    expect(screen.queryByText('你是专业翻译。')).not.toBeInTheDocument()

    fireEvent.click(screen.getByRole('button', { name: '编辑 翻译官' }))
    await waitFor(() => {
      expect(screen.getByLabelText('系统提示词（仅编辑时可见）')).toHaveValue('你是专业翻译。')
    })
    expect(mockGetAgent).toHaveBeenCalledWith('agent-1')
  })

  it('updates an existing agent', async () => {
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: '编辑 翻译官' }))
    await waitFor(() => {
      expect(screen.getByLabelText('系统提示词（仅编辑时可见）')).toHaveValue('你是专业翻译。')
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
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: '删除 翻译官' }))

    await waitFor(() => {
      expect(mockDeleteAgent).toHaveBeenCalledWith('agent-1')
    })
  })

  it('does not offer delete for builtin main agent', async () => {
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    expect(screen.queryByRole('button', { name: '删除 主智能体' })).not.toBeInTheDocument()
  })

  it('shows error when delete fails', async () => {
    mockDeleteAgent.mockRejectedValue(new Error('删除失败：内置智能体不可删除'))
    render(<AgentsManager />)
    await screen.findByText('翻译官')

    fireEvent.click(screen.getByRole('button', { name: '删除 翻译官' }))

    await waitFor(() => {
      expect(screen.getByText(/删除失败/)).toBeInTheDocument()
    })
  })
})
