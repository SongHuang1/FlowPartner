interface FlowPartnerAPI {
  platform: NodeJS.Platform
  getVersion: () => Promise<string>
  onSystemLock: (callback: () => void) => void
  onSystemFocus: (callback: () => void) => void
  fetchBackendPort: () => Promise<number>
  onCloseAction: (callback: () => void) => void
  sendCloseAction: (action: 'minimize' | 'quit') => void
  updateCloseBehavior: (behavior: string, remembered: boolean) => void
  openExternal: (url: string) => Promise<void>
  selectFolder: () => Promise<string | null>
}

interface Window {
  flowPartner: FlowPartnerAPI
}
