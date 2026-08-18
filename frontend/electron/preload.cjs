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
    onCloseAction: (callback) => {
        ipcRenderer.on('request-close-action', () => callback())
    },
    sendCloseAction: (action) => {
        ipcRenderer.send('close-action-response', action)
    },
    updateCloseBehavior: (behavior, remembered) => {
        ipcRenderer.send('update-close-behavior', behavior, remembered)
    },
    openExternal: (url) => ipcRenderer.invoke('open-external', url),
})
