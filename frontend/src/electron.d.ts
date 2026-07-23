interface FlowPartnerAPI {
  platform: NodeJS.Platform
  getVersion: () => Promise<string>
  onSystemLock: (callback: () => void) => void
}

interface Window {
  flowPartner: FlowPartnerAPI
}
