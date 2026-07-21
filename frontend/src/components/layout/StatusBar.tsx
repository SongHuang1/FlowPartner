export function StatusBar() {
  const isElectron = typeof window !== 'undefined' && window.flowPartner

  return (
    <div className="h-6 flex items-center px-3 border-t border-neutral-200 bg-neutral-50 text-xs text-neutral-500 shrink-0">
      {isElectron ? '桌面端 · FlowPartner' : '浏览器中运行 · 仅 UI 预览'}
    </div>
  )
}
