import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor, within } from '@testing-library/react'
import { APISettings } from '@/components/settings/APISettings'
import type { LockStatus } from '@/types'

const mockSettings = {
  model: 'gpt-4',
  agent_id: 'default',
  context_window: 8192,
  working_directory: '',
  trash_dir: '',
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
  model_configs: [] as Array<Record<string, unknown>>,
  active_config_id: '',
}

const mockUpdateSettings = vi.fn()
const mockGetCurrentSettings = vi.fn()
const mockRefreshSettings = vi.fn()
const mockLockStatus: LockStatus = { locked: true, failed_attempts: 0, has_api_key: false }
const mockUnlock = vi.fn()
const mockLock = vi.fn()
const mockRefreshStatus = vi.fn()
const mockSaveSettings = vi.fn()
const mockClearApiKey = vi.fn()

vi.mock('@/hooks/useSettings', () => ({
  useSettings: () => ({
    settings: mockSettings,
    updateSettings: mockUpdateSettings,
    getCurrentSettings: mockGetCurrentSettings,
    refreshSettings: mockRefreshSettings,
  }),
}))

vi.mock('@/hooks/useLock', () => ({
  useLock: () => ({
    lockStatus: mockLockStatus,
    unlock: mockUnlock,
    lock: mockLock,
    refreshStatus: mockRefreshStatus,
  }),
}))

vi.mock('@/lib/api', () => ({
  saveSettings: (s: unknown) => mockSaveSettings(s),
  clearApiKey: () => mockClearApiKey(),
}))

vi.mock('@/lib/validation', () => ({
  isPasswordStrong: (pw: string) =>
    pw.length >= 8 && /[A-Z]/.test(pw) && /[a-z]/.test(pw) && /[0-9]/.test(pw),
}))

const configOne = {
  id: 'cfg-1',
  name: 'OpenAI 主账号',
  base_url: 'https://api.openai.com/v1',
  model_name: 'gpt-4',
  temperature: 0.7,
  response_format: 'text',
  timeout_secs: 30,
}

beforeEach(() => {
  vi.clearAllMocks()
  mockLockStatus.locked = true
  mockLockStatus.failed_attempts = 0
  mockLockStatus.has_api_key = false
  mockSettings.model_configs = []
  mockSettings.active_config_id = ''
  mockGetCurrentSettings.mockReturnValue(mockSettings)
  mockRefreshSettings.mockResolvedValue(undefined)
  mockRefreshStatus.mockResolvedValue(undefined)
  mockUnlock.mockResolvedValue(undefined)
  mockLock.mockResolvedValue(undefined)
  mockSaveSettings.mockResolvedValue(undefined)
  mockClearApiKey.mockResolvedValue(undefined)
})

describe('APISettings - Mode A: no API key configured', () => {
  it('renders 新建 API 配置 form', () => {
    render(<APISettings />)
    expect(screen.getByText('新建 API 配置')).toBeInTheDocument()
    expect(screen.getByLabelText('配置名称')).toBeInTheDocument()
    expect(screen.getByLabelText('模型名称')).toBeInTheDocument()
    expect(screen.getByLabelText('接口地址')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('输入 API Key')).toBeInTheDocument()
    expect(screen.getByLabelText('保护密码')).toBeInTheDocument()
    expect(screen.getByLabelText('确认密码')).toBeInTheDocument()
  })

  it('displays current base_url and model values', () => {
    render(<APISettings />)
    expect((screen.getByLabelText('接口地址') as HTMLInputElement).value).toBe('https://api.openai.com/v1')
    expect((screen.getByLabelText('模型名称') as HTMLInputElement).value).toBe('gpt-4')
  })

  it('save button is disabled when required fields are empty', () => {
    render(<APISettings />)
    expect(screen.getByText('保存配置')).toBeDisabled()
  })

  it('first-time save calls saveSettings with api key and password', async () => {
    render(<APISettings />)
    fireEvent.change(screen.getByLabelText('配置名称'), { target: { value: 'OpenAI 主账号' } })
    fireEvent.change(screen.getByPlaceholderText('输入 API Key'), { target: { value: 'sk-test-123' } })
    fireEvent.change(screen.getByLabelText('保护密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'TestPass123' } })

    fireEvent.click(screen.getByText('保存配置'))

    await waitFor(() => {
      expect(mockSaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ api_key: 'sk-test-123', password: 'TestPass123' }),
      )
    })
    expect(mockRefreshSettings).toHaveBeenCalled()
    expect(mockRefreshStatus).toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText('API Key 配置成功')).toBeInTheDocument()
    })
  })

  it('shows weak password error on first-time save', async () => {
    render(<APISettings />)
    fireEvent.change(screen.getByLabelText('配置名称'), { target: { value: 'OpenAI 主账号' } })
    fireEvent.change(screen.getByPlaceholderText('输入 API Key'), { target: { value: 'sk-test-123' } })
    fireEvent.change(screen.getByLabelText('保护密码'), { target: { value: 'weak123' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'weak123' } })

    fireEvent.click(screen.getByText('保存配置'))

    await waitFor(() => {
      expect(screen.getByText('密码至少 8 位，且需包含大写字母、小写字母和数字')).toBeInTheDocument()
    })
    expect(mockSaveSettings).not.toHaveBeenCalled()
  })

  it('shows inline weak password hint', () => {
    render(<APISettings />)
    fireEvent.change(screen.getByLabelText('保护密码'), { target: { value: 'abc' } })
    expect(screen.getByText('需包含大写、小写字母和数字')).toBeInTheDocument()
  })

  it('shows inline password mismatch hint', () => {
    render(<APISettings />)
    fireEvent.change(screen.getByLabelText('保护密码'), { target: { value: 'TestPass123' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'Different1' } })
    expect(screen.getByText('密码不一致')).toBeInTheDocument()
  })

  it('toggles API Key visibility with eye button', () => {
    render(<APISettings />)
    const input = screen.getByPlaceholderText('输入 API Key') as HTMLInputElement
    expect(input.type).toBe('password')
    const toggle = input.closest('div')!.querySelector('button')!
    fireEvent.click(toggle)
    expect((screen.getByPlaceholderText('输入 API Key') as HTMLInputElement).type).toBe('text')
  })
})

describe('APISettings - Mode B: locked with API key', () => {
  beforeEach(() => {
    mockLockStatus.has_api_key = true
  })

  it('renders unlock UI', () => {
    render(<APISettings />)
    expect(screen.getByText('API Key 已加密')).toBeInTheDocument()
    expect(screen.getByPlaceholderText('输入密码解锁')).toBeInTheDocument()
    expect(screen.getByText('解锁')).toBeInTheDocument()
  })

  it('calls unlock with password when unlock button clicked', async () => {
    render(<APISettings />)
    fireEvent.change(screen.getByPlaceholderText('输入密码解锁'), { target: { value: 'TestPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    await waitFor(() => {
      expect(mockUnlock).toHaveBeenCalledWith('TestPass123')
    })
  })

  it('shows error when unlock clicked with empty password', () => {
    render(<APISettings />)
    fireEvent.click(screen.getByText('解锁'))
    expect(screen.getByText('请输入密码')).toBeInTheDocument()
  })

  it('shows error when unlock fails', async () => {
    mockUnlock.mockRejectedValue(new Error('Wrong password'))
    render(<APISettings />)
    fireEvent.change(screen.getByPlaceholderText('输入密码解锁'), { target: { value: 'WrongPass123' } })
    fireEvent.click(screen.getByText('解锁'))

    await waitFor(() => {
      expect(screen.getByText('Wrong password')).toBeInTheDocument()
    })
  })

  it('lists saved configs when locked', () => {
    mockSettings.model_configs = [configOne]
    mockSettings.active_config_id = 'cfg-1'
    render(<APISettings />)
    expect(screen.getByText('已保存的配置')).toBeInTheDocument()
    expect(screen.getByText('OpenAI 主账号')).toBeInTheDocument()
    expect(screen.getByText('https://api.openai.com/v1')).toBeInTheDocument()
  })
})

describe('APISettings - Mode C: unlocked with API key', () => {
  beforeEach(() => {
    mockLockStatus.has_api_key = true
    mockLockStatus.locked = false
  })

  it('renders model config and edit key sections', () => {
    render(<APISettings />)
    expect(screen.getByText('模型配置')).toBeInTheDocument()
    expect(screen.getByText('修改当前密钥')).toBeInTheDocument()
    expect(screen.getByText('修改并重新加密')).toBeInTheDocument()
    expect(screen.getByText('清除')).toBeInTheDocument()
    expect(screen.getByText('锁定')).toBeInTheDocument()
  })

  it('calls lock when lock button clicked', async () => {
    render(<APISettings />)
    fireEvent.click(screen.getByText('锁定'))
    await waitFor(() => {
      expect(mockLock).toHaveBeenCalled()
    })
  })

  it('saves a new API key with password', async () => {
    render(<APISettings />)
    fireEvent.change(screen.getByLabelText('新 API Key'), { target: { value: 'sk-new-key' } })
    fireEvent.change(screen.getByLabelText('保护密码'), { target: { value: 'NewPass123' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'NewPass123' } })
    fireEvent.click(screen.getByText('修改并重新加密'))

    await waitFor(() => {
      expect(mockSaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({ api_key: 'sk-new-key', password: 'NewPass123' }),
      )
    })
    expect(mockRefreshSettings).toHaveBeenCalled()
    expect(mockRefreshStatus).toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText('API Key 已更新')).toBeInTheDocument()
    })
  })

  it('shows weak password error when editing API key', async () => {
    render(<APISettings />)
    fireEvent.change(screen.getByLabelText('新 API Key'), { target: { value: 'sk-new-key' } })
    fireEvent.change(screen.getByLabelText('保护密码'), { target: { value: 'weak123' } })
    fireEvent.change(screen.getByLabelText('确认密码'), { target: { value: 'weak123' } })
    fireEvent.click(screen.getByText('修改并重新加密'))

    await waitFor(() => {
      expect(screen.getByText('密码至少 8 位，且需包含大写字母、小写字母和数字')).toBeInTheDocument()
    })
    expect(mockSaveSettings).not.toHaveBeenCalled()
  })

  it('clears API key when clear button clicked', async () => {
    render(<APISettings />)
    fireEvent.click(screen.getByText('清除'))

    await waitFor(() => {
      expect(mockClearApiKey).toHaveBeenCalled()
    })
    expect(mockRefreshSettings).toHaveBeenCalled()
    expect(mockRefreshStatus).toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText('API Key 已清除')).toBeInTheDocument()
    })
  })

  it('adds a new config via the form', async () => {
    render(<APISettings />)
    fireEvent.click(screen.getByText('新增配置'))
    const form = screen.getByText('新增配置').closest('.rounded-lg') as HTMLElement
    fireEvent.change(within(form).getByLabelText('名称'), { target: { value: 'Claude' } })
    fireEvent.change(within(form).getByLabelText('模型'), { target: { value: 'claude-3' } })
    fireEvent.change(within(form).getByLabelText('接口地址'), { target: { value: 'https://api.anthropic.com/v1' } })
    fireEvent.change(within(form).getByLabelText('API Key'), { target: { value: 'sk-claude' } })
    fireEvent.change(within(form).getByLabelText('保护密码'), { target: { value: 'ClaudePass1' } })
    fireEvent.change(within(form).getByLabelText('确认密码'), { target: { value: 'ClaudePass1' } })
    fireEvent.click(within(form).getByText('保存'))

    await waitFor(() => {
      expect(mockSaveSettings).toHaveBeenCalledWith(
        expect.objectContaining({
          model: 'claude-3',
          base_url: 'https://api.anthropic.com/v1',
          api_key: 'sk-claude',
          password: 'ClaudePass1',
          model_configs: expect.arrayContaining([
            expect.objectContaining({ name: 'Claude', model_name: 'claude-3' }),
          ]),
          active_config_id: expect.any(String),
        }),
      )
    })
    await waitFor(() => {
      expect(screen.getByText('新配置已添加')).toBeInTheDocument()
    })
  })

  it('shows inline validation hints in the add-config form', () => {
    render(<APISettings />)
    fireEvent.click(screen.getByText('新增配置'))
    const form = screen.getByText('新增配置').closest('.rounded-lg') as HTMLElement
    fireEvent.change(within(form).getByLabelText('保护密码'), { target: { value: 'abc' } })
    expect(screen.getByText('需包含大写、小写字母和数字')).toBeInTheDocument()
    fireEvent.change(within(form).getByLabelText('保护密码'), { target: { value: 'ClaudePass1' } })
    fireEvent.change(within(form).getByLabelText('确认密码'), { target: { value: 'Different1' } })
    expect(screen.getByText('密码不一致')).toBeInTheDocument()
  })

  it('deletes a config after confirming', async () => {
    mockSettings.model_configs = [configOne]
    mockSettings.active_config_id = 'cfg-1'
    render(<APISettings />)

    const row = screen.getByText('OpenAI 主账号').closest('.rounded-lg')!
    fireEvent.click(within(row as HTMLElement).getByRole('button'))
    fireEvent.click(screen.getByText('确认删除'))

    expect(mockUpdateSettings).toHaveBeenCalledWith(
      expect.objectContaining({ model_configs: [], active_config_id: '' }),
    )
    expect(mockRefreshStatus).toHaveBeenCalled()
    await waitFor(() => {
      expect(screen.getByText('配置已删除')).toBeInTheDocument()
    })
  })

  it('clicking a config row sets it active', () => {
    mockSettings.model_configs = [configOne]
    render(<APISettings />)
    fireEvent.click(screen.getByText('OpenAI 主账号'))
    expect(mockUpdateSettings).toHaveBeenCalledWith({ active_config_id: 'cfg-1' })
  })
})