import { useState } from 'react'
import { useSettings } from '@/hooks/useSettings'
import type { InputHTMLAttributes } from 'react'
import { FolderOpen } from 'lucide-react'

const inputClass = 'flex w-full rounded-lg border border-neutral-200 bg-white px-4 py-2.5 text-sm shadow-sm placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400'

function Field({ className, ...props }: InputHTMLAttributes<HTMLInputElement>) {
  return <input className={inputClass + ' ' + (className || '')} {...props} />
}

function ContextField({ value, onChange }: { value: number; onChange: (v: number) => void }) {
  const [local, setLocal] = useState(String(value))
  const [prevValue, setPrevValue] = useState(value)
  if (prevValue !== value) {
    setPrevValue(value)
    setLocal(String(value))
  }

  const handleBlur = () => {
    const parsed = parseInt(local, 10)
    if (isNaN(parsed) || parsed < 1) {
      onChange(1)
      setLocal('1')
    } else {
      onChange(parsed)
      setLocal(String(parsed))
    }
  }

  return (
    <Field
      id="context-window"
      type="text"
      inputMode="numeric"
      value={local}
      onChange={(e) => setLocal(e.target.value)}
      onBlur={handleBlur}
      placeholder="32768"
    />
  )
}

export function AgentSettings() {
  const { settings, updateSettings } = useSettings()

  const handleSelectFolder = async () => {
    const folder = await window.flowPartner.selectFolder()
    if (folder) {
      updateSettings({ working_directory: folder })
    }
  }

  const handleSelectTrashFolder = async () => {
    const folder = await window.flowPartner.selectFolder()
    if (folder) {
      updateSettings({ trash_dir: folder })
    }
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">基础设置</h4>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="agent-id" className="text-xs font-medium text-neutral-600">智能体 ID</label>
          <Field
            id="agent-id"
            value={settings.agent_id}
            onChange={(e) => updateSettings({ agent_id: e.target.value })}
            placeholder="default"
          />
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="working-dir" className="text-xs font-medium text-neutral-600">工作文件夹</label>
          <div className="flex gap-2">
            <Field
              id="working-dir"
              value={settings.working_directory}
              onChange={(e) => updateSettings({ working_directory: e.target.value })}
              placeholder="选择 Agent 可访问的文件夹"
              className="flex-1"
            />
            <button
              type="button"
              onClick={handleSelectFolder}
              className="px-3 py-2 bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 rounded-lg transition-colors"
              title="浏览文件夹"
            >
              <FolderOpen className="w-4 h-4 text-neutral-600" />
            </button>
          </div>
          <p className="text-xs text-neutral-400">限制 Agent 的文件操作范围于此文件夹内</p>
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="trash-dir" className="text-xs font-medium text-neutral-600">回收站文件夹</label>
          <div className="flex gap-2">
            <Field
              id="trash-dir"
              value={settings.trash_dir}
              onChange={(e) => updateSettings({ trash_dir: e.target.value })}
              placeholder="选择回收站文件夹"
              className="flex-1"
            />
            <button
              type="button"
              onClick={handleSelectTrashFolder}
              className="px-3 py-2 bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 rounded-lg transition-colors"
              title="浏览文件夹"
            >
              <FolderOpen className="w-4 h-4 text-neutral-600" />
            </button>
          </div>
          <p className="text-xs text-neutral-400">Agent 删除的文件将移入此文件夹，而非永久删除</p>
        </div>
      </div>

      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">对话参数</h4>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="system-prompt" className="text-xs font-medium text-neutral-600">系统提示词</label>
          <textarea
            id="system-prompt"
            value={settings.system_prompt}
            onChange={(e) => updateSettings({ system_prompt: e.target.value })}
            placeholder="你是一个乐于助人的 AI 助手。"
            rows={5}
            className="flex w-full rounded-lg border border-neutral-200 bg-white px-4 py-3 text-sm shadow-sm placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 resize-none"
          />
        </div>

        <div className="flex flex-col gap-3 p-4 bg-neutral-50 rounded-lg border border-neutral-100">
          <div className="flex items-center justify-between">
            <label htmlFor="temperature" className="text-xs font-medium text-neutral-600">温度</label>
            <span className="text-sm font-mono font-medium text-blue-600 bg-blue-50 px-2 py-0.5 rounded">
              {settings.temperature.toFixed(1)}
            </span>
          </div>
          <input
            id="temperature"
            type="range"
            min="0"
            max="1"
            step="0.1"
            value={settings.temperature}
            onChange={(e) => updateSettings({ temperature: parseFloat(e.target.value) })}
            className="w-full accent-blue-500"
          />
          <div className="flex justify-between text-xs text-neutral-400">
            <span>0.0 精确</span>
            <span>0.7 平衡</span>
            <span>1.0 创意</span>
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <label htmlFor="context-window" className="text-xs font-medium text-neutral-600">上下文窗口</label>
          <ContextField value={settings.context_window} onChange={(v) => updateSettings({ context_window: v })} />
          <p className="text-xs text-neutral-400">单位：tokens，可自由输入任意正整数（如 1000000 = 1M）</p>
        </div>
      </div>
    </div>
  )
}


