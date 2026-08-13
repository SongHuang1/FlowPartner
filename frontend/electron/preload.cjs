const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('flowPartner', {
    platform: process.platform,
    getVersion: () => ipcRenderer.invoke('get-app-version'),
    onSystemLock: (callback) => {
        ipcRenderer.on('system-lock', () => callback())
    },
    fetchBackendPort: () => ipcRenderer.invoke('get-backend-port'),
    onBackendPortChanged: (callback) => {
        const listener = (_, port) => callback(port)
        ipcRenderer.on('backend-port-changed', listener)
        return () => ipcRenderer.removeListener('backend-port-changed', listener)
    },
})
