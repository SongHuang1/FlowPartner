const { app, BrowserWindow, dialog, Tray, Menu, ipcMain } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')

let goProcess = null
let mainWindow = null
let tray = null
let backendPort = null
let isQuiting = false

function getBackendBinPath() {
  if (process.env.FP_BINARY_PATH) {
    return process.env.FP_BINARY_PATH
  }
  const exeName = process.platform === 'win32' ? 'flowpartner-backend.exe' : 'flowpartner-backend'
  return path.join(process.resourcesPath, 'bin', exeName)
}

function findFreePort() {
  return new Promise((resolve, reject) => {
    const server = http.createServer()
    server.listen(0, () => {
      const port = server.address().port
      server.close(() => resolve(port))
    })
    server.on('error', reject)
  })
}

function startGoProcess(port) {
  const isDev = !app.isPackaged || process.env.ELECTRON_DEV === 'true'

  if (isDev) {
    goProcess = spawn('go', ['run', 'cmd/server/main.go'], {
      env: { ...process.env, FP_DEV_MODE: 'true' },
      cwd: path.join(__dirname, '..', '..', 'backend'),
      shell: true,
    })
  } else {
    const binPath = getBackendBinPath()
    goProcess = spawn(binPath, [], {
      env: { ...process.env, FP_HTTP_PORT: `:${port}` },
    })
  }

  goProcess.stderr.on('data', (data) => {
    const output = data.toString()
    process.stderr.write(output)

    if (output.includes('__FP_BACKEND_READY__') && backendPort === null) {
      backendPort = port
    }
  })

  goProcess.on('error', (err) => {
    if (!isQuiting) {
      dialog.showErrorBox('启动失败', '开发环境未检测到 Go 编译器，请先安装 Go（https://go.dev/dl/）')
      app.quit()
    }
  })

  goProcess.on('exit', (code) => {
    if (code !== 0 && !isQuiting) {
      dialog.showErrorBox('后端异常退出', `Go 后端进程已退出，退出码：${code}`)
    }
    goProcess = null
  })
}

function waitForReady(timeoutMs) {
  return new Promise((resolve, reject) => {
    const start = Date.now()

    if (!goProcess) {
      reject(new Error('Go process not started'))
      return
    }

    const checkReady = () => {
      if (backendPort !== null) {
        resolve(backendPort)
        return
      }
      if (Date.now() - start > timeoutMs) {
        reject(new Error('Backend ready timeout'))
        return
      }
      setTimeout(checkReady, 100)
    }

    checkReady()
  })
}

function stopGoProcess() {
  if (!goProcess) return Promise.resolve()

  return new Promise((resolve) => {
    const pid = goProcess.pid
    const timeout = setTimeout(() => {
      try { process.kill(pid, 'SIGKILL') } catch { /* already dead */ }
      goProcess = null
      resolve()
    }, 3000)

    goProcess.on('exit', () => {
      clearTimeout(timeout)
      goProcess = null
      resolve()
    })

    try { process.kill(pid, 'SIGTERM') } catch { /* already dead */ }
  })
}

function showMainWindow() {
  if (mainWindow) {
    mainWindow.show()
    mainWindow.focus()
  } else if (goProcess && backendPort) {
    createWindow(backendPort)
  } else {
    dialog.showErrorBox('后端已退出', '后端服务已停止运行，请重启应用。')
  }
}

function quitApp() {
  isQuiting = true
  app.quit()
}

function createWindow(port) {
  mainWindow = new BrowserWindow({
    width: 1200,
    height: 800,
    minWidth: 800,
    minHeight: 600,
    title: 'FlowPartner',
    webPreferences: {
      preload: path.join(__dirname, 'preload.cjs'),
      contextIsolation: true,
      nodeIntegration: false,
      sandbox: true,
    },
  })

  const isDev = !app.isPackaged || process.env.ELECTRON_DEV === 'true'

  if (isDev) {
    mainWindow.loadURL('http://localhost:5173')
  } else {
    mainWindow.loadURL(`http://localhost:${port}`)
  }

  mainWindow.on('close', (event) => {
    if (!isQuiting) {
      event.preventDefault()
      mainWindow.hide()
    }
  })

  mainWindow.on('closed', () => {
    mainWindow = null
  })
}

function createTray() {
  const iconPath = path.join(__dirname, 'tray-icon.png')
  tray = new Tray(iconPath)
  tray.setToolTip('FlowPartner')

  const contextMenu = Menu.buildFromTemplate([
    {
      label: '显示主窗口',
      click: showMainWindow,
    },
    { type: 'separator' },
    {
      label: '退出 FlowPartner',
      click: quitApp,
    },
  ])

  tray.setContextMenu(contextMenu)

  tray.on('double-click', showMainWindow)
}

function createApplicationMenu() {
  const template = [
    {
      label: '编辑',
      submenu: [
        { role: 'undo', label: '撤销' },
        { role: 'redo', label: '重做' },
        { type: 'separator' },
        { role: 'cut', label: '剪切' },
        { role: 'copy', label: '复制' },
        { role: 'paste', label: '粘贴' },
        { role: 'selectAll', label: '全选' },
      ],
    },
    {
      label: '视图',
      submenu: [
        { role: 'reload', label: '刷新' },
        { role: 'forceReload', label: '强制刷新' },
        { role: 'toggleDevTools', label: '切换开发者工具' },
        { type: 'separator' },
        { role: 'resetZoom', label: '实际大小' },
        { role: 'zoomIn', label: '放大' },
        { role: 'zoomOut', label: '缩小' },
      ],
    },
    {
      label: '窗口',
      submenu: [
        { role: 'minimize', label: '最小化' },
        { role: 'close', label: '关闭' },
      ],
    },
    {
      label: '帮助',
      submenu: [
        {
          label: '关于 FlowPartner',
          click: () => {
            dialog.showMessageBox({
              type: 'info',
              title: '关于 FlowPartner',
              message: 'FlowPartner',
              detail: `版本 ${app.getVersion()}\n\nAI 助手桌面应用，为非技术用户提供安全可靠的智能助理服务。`,
            })
          },
        },
      ],
    },
  ]

  const menu = Menu.buildFromTemplate(template)
  Menu.setApplicationMenu(menu)
}

ipcMain.handle('get-app-version', () => {
  return app.getVersion()
})

app.whenReady().then(async () => {
  try {
    createTray()
    createApplicationMenu()
  } catch (err) {
    dialog.showErrorBox('启动失败', `初始化失败：${err.message}`)
    app.quit()
    return
  }

  const isDev = !app.isPackaged || process.env.ELECTRON_DEV === 'true'

  let port
  if (isDev) {
    port = 8080
    try {
      const testServer = http.createServer()
      await new Promise((resolve, reject) => {
        testServer.once('error', reject)
        testServer.once('listening', () => {
          testServer.close()
          resolve()
        })
        testServer.listen(port)
      })
    } catch {
      port = await findFreePort()
    }
  } else {
    port = await findFreePort()
  }

  startGoProcess(port)

  try {
    const readyPort = await waitForReady(10000)
    createWindow(readyPort)
  } catch (err) {
    dialog.showErrorBox('启动失败', '无法启动后端服务，请重启应用。')
    app.quit()
  }
})

app.on('before-quit', async () => {
  isQuiting = true
  if (tray) {
    tray.destroy()
    tray = null
  }
  await stopGoProcess()
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    isQuiting = true
    app.quit()
  }
})
