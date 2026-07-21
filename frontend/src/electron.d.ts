interface FlowPartnerAPI {
  platform: NodeJS.Platform
  getVersion: () => Promise<string>
}

interface Window {
  flowPartner: FlowPartnerAPI
}
