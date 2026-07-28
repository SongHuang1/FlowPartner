const { contextBridge, ipcRenderer } = require('electron')

contextBridge.exposeInMainWorld('flowPartner', {
    platform: process.platform,
    getVersion: () => ipcRenderer.invoke('get-app-version'),
    onSystemLock: (callback) => {
        ipcRenderer.on('system-lock', () => callback())
    },
})
