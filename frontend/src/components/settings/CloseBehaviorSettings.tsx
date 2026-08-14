import { useSettings } from '@/hooks/useSettings'

export function CloseBehaviorSettings() {
  const { settings, updateSettings } = useSettings()

  const options: { value: 'ask' | 'minimize' | 'quit'; label: string; desc: string; icon: string }[] = [
    { value: 'ask', label: '每次询问', desc: '关闭时弹出选择框', icon: '?' },
    { value: 'minimize', label: '最小化到托盘', desc: '关闭窗口时后台运行', icon: '—' },
    { value: 'quit', label: '完全退出', desc: '关闭窗口时结束程序', icon: '×' },
  ]

  const handleChange = (value: 'ask' | 'minimize' | 'quit') => {
    updateSettings({
      close_behavior: value,
      close_remembered: value !== 'ask',
    })
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-3">
        <h4 className="text-sm font-medium text-neutral-700">关闭行为</h4>
        <p className="text-xs text-neutral-500">选择点击关闭窗口按钮后的默认行为</p>
        <div className="flex flex-col gap-2 mt-1">
          {options.map(({ value, label, desc, icon }) => (
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
                <div className="flex items-center gap-2">
                  <span className={
                    'w-5 h-5 rounded-full flex items-center justify-center text-xs font-medium ' +
                    (settings.close_behavior === value ? 'bg-blue-500 text-white' : 'bg-neutral-200 text-neutral-600')
                  }>
                    {icon}
                  </span>
                  <span className="text-sm font-medium text-neutral-700">{label}</span>
                </div>
                <span className="text-xs text-neutral-500 ml-7">{desc}</span>
              </div>
            </label>
          ))}
        </div>
      </div>
    </div>
  )
}
