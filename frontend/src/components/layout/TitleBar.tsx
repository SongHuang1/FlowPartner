export function TitleBar() {
  return (
    <div className="h-11 flex items-center px-4 border-b border-neutral-200 bg-neutral-50 shrink-0">
      <span className="font-semibold text-sm text-neutral-800">FlowPartner</span>
      <span className="ml-3 flex items-center">
        <span className="w-2 h-2 rounded-full bg-green-500 inline-block" />
      </span>
      <span className="ml-2 text-xs text-neutral-500">UI Shell</span>
    </div>
  )
}
