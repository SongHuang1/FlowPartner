import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { initApi } from './lib/api'
import { SettingsProvider } from './hooks/useSettings'
import { LockProvider } from './hooks/useLock'

async function bootstrap() {
  const port = await window.flowPartner.fetchBackendPort()
  initApi(port)

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <SettingsProvider>
        <LockProvider>
          <App />
        </LockProvider>
      </SettingsProvider>
    </StrictMode>,
  )
}

bootstrap().catch((err) => {
  console.error('Failed to start application:', err)
  const root = document.getElementById('root')
  if (root) {
    root.innerHTML = '<div style="padding:20px;text-align:center;">后端启动失败，请重启应用</div>'
  }
})
