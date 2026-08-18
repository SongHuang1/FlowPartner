interface FlowPartnerAPI {
  platform: NodeJS.Platform
  getVersion: () => Promise<string>
  onSystemLock: (callback: () => void) => void
  fetchBackendPort: () => Promise<number>
  onBackendPortChanged: (callback: (port: number) => void) => () => void
  onCloseAction: (callback: () => void) => void
  sendCloseAction: (action: 'minimize' | 'quit') => void
  updateCloseBehavior: (behavior: string, remembered: boolean) => void
  openExternal: (url: string) => Promise<void>
}

interface Window {
  flowPartner: FlowPartnerAPI
}
