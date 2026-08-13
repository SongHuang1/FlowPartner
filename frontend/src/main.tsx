import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'
import { initApi } from './lib/api'

async function bootstrap() {
  const port = await window.flowPartner.fetchBackendPort()
  initApi(port)

  createRoot(document.getElementById('root')!).render(
    <StrictMode>
      <App />
    </StrictMode>,
  )
}

bootstrap().catch((err) => {
  console.error('Failed to start application:', err)
  const root = document.getElementById('root')
  if (root) {
    root.innerHTML = '<div style="padding:20px;text-align:center;">后端服务启动失败，请重启应用</div>'
  }
})
