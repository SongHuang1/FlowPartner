export function StatusBar() {
  const isElectron = typeof window !== 'undefined' && window.flowPartner

  return (
    <div className="h-6 flex items-center px-3 border-t border-neutral-200 bg-neutral-50 text-xs text-neutral-500 shrink-0">
      {isElectron ? 'Desktop · FlowPartner' : 'Running in browser · UI preview only'}
    </div>
  )
}
