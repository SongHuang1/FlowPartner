interface FlowPartnerAPI {
  platform: NodeJS.Platform
  getVersion: () => Promise<string>
  onSystemLock: (callback: () => void) => void
  fetchBackendPort: () => Promise<number>
  onBackendPortChanged: (callback: (port: number) => void) => () => void
}

interface Window {
  flowPartner: FlowPartnerAPI
}
