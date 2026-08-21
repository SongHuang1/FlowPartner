const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('flowPartner', {
    platform: process.platform,
    getVersion: () => ipcRenderer.invoke('get-app-version'),
    onSystemLock: (callback) => {
        ipcRenderer.on('system-lock', () => callback())
    },
    onSystemFocus: (callback) => {
        ipcRenderer.on('system-focus', () => callback())
    },
    fetchBackendPort: () => ipcRenderer.invoke('get-backend-port'),
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
    selectFolder: () => ipcRenderer.invoke('select-folder'),
})
