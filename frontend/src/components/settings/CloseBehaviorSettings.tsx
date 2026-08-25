import { useSettings } from '@/hooks/useSettings'
import { useEffect } from 'react'
import { FolderOpen } from 'lucide-react'

const inputClass = 'flex w-full rounded-lg border border-neutral-200 bg-white px-4 py-2.5 text-sm shadow-sm placeholder:text-neutral-400 focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400'

export function CloseBehaviorSettings() {
  const { settings, updateSettings } = useSettings()

  useEffect(() => {
    window.flowPartner.updateCloseBehavior(settings.close_behavior, settings.close_remembered)
  }, [settings.close_behavior, settings.close_remembered])

  const handleChange = (value: 'ask' | 'minimize' | 'quit') => {
    const remembered = value !== 'ask'
    updateSettings({
      close_behavior: value,
      close_remembered: remembered,
    })
    window.flowPartner.updateCloseBehavior(value, remembered)
  }

  const handleSelectFolder = async (field: 'working_directory' | 'trash_dir') => {
    const folder = await window.flowPartner.selectFolder()
    if (folder) {
      updateSettings({ [field]: folder })
    }
  }

  const options: { value: 'ask' | 'minimize' | 'quit'; label: string; desc: string }[] = [
    { value: 'ask', label: '每次询问', desc: '关闭时弹出选择框' },
    { value: 'minimize', label: '最小化到托盘', desc: '关闭窗口时后台运行' },
    { value: 'quit', label: '完全退出', desc: '关闭窗口时结束程序' },
  ]

  return (
    <div className="flex flex-col gap-6">
      {/* 关闭行为 */}
      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">关闭行为</h4>
        <p className="text-xs text-neutral-500">选择点击关闭窗口按钮后的默认行为</p>
        <div className="flex flex-col gap-2 mt-1">
          {options.map(({ value, label, desc }) => (
            <label
              key={value}
              className={
                'flex items-start gap-4 p-4 rounded-lg border transition-colors cursor-pointer ' +
                (settings.close_behavior === value
                  ? 'border-blue-300 bg-blue-50/30 shadow-sm'
                  : 'border-neutral-200 hover:border-neutral-300 hover:bg-neutral-50')
              }
              onClick={() => handleChange(value)}
            >
              <input
                type="radio"
                name="close-behavior"
                checked={settings.close_behavior === value}
                onChange={() => handleChange(value)}
                className="mt-0.5 accent-blue-500"
              />
              <div className="flex flex-col gap-1">
                <span className="text-sm font-medium text-neutral-700">{label}</span>
                <span className="text-xs text-neutral-500">{desc}</span>
              </div>
            </label>
          ))}
        </div>
      </div>

      {/* 分隔线 */}
      <div className="border-t border-neutral-200" />

      {/* 文件夹设置 */}
      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">文件夹</h4>

        <div className="flex flex-col gap-1.5">
          <label htmlFor="working-dir" className="text-xs font-medium text-neutral-600">工作文件夹</label>
          <div className="flex gap-2">
            <input
              id="working-dir"
              value={settings.working_directory}
              onChange={(e) => updateSettings({ working_directory: e.target.value })}
              placeholder="选择 Agent 可访问的文件夹"
              className={inputClass + ' flex-1'}
            />
            <button
              type="button"
              onClick={() => handleSelectFolder('working_directory')}
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
            <input
              id="trash-dir"
              value={settings.trash_dir}
              onChange={(e) => updateSettings({ trash_dir: e.target.value })}
              placeholder="选择回收站文件夹"
              className={inputClass + ' flex-1'}
            />
            <button
              type="button"
              onClick={() => handleSelectFolder('trash_dir')}
              className="px-3 py-2 bg-neutral-100 hover:bg-neutral-200 border border-neutral-200 rounded-lg transition-colors"
              title="浏览文件夹"
            >
              <FolderOpen className="w-4 h-4 text-neutral-600" />
            </button>
          </div>
          <p className="text-xs text-neutral-400">Agent 删除的文件将移入此文件夹，而非永久删除</p>
        </div>
      </div>
    </div>
  )
}
