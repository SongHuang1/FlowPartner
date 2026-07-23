const { app, BrowserWindow, dialog, Tray, Menu, ipcMain, powerMonitor, screen } = require('electron')
const { spawn } = require('child_process')
const path = require('path')
const http = require('http')
const fs = require('fs')
const crypto = require('crypto')

let goProcess = null
let pythonProcess = null
let mainWindow = null
let tray = null
let backendPort = null
let isQuiting = false
let agentAuthToken = null
let cleanupDone = false

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

function generateAuthToken() {
  return crypto.randomBytes(32).toString('hex')
}

function filterSafeEnv(env) {
  const allowedKeys = [
    'PATH', 'HOME', 'USERPROFILE', 'SYSTEMROOT', 'COMSPEC',
    'LANG', 'LC_ALL', 'LC_CTYPE', 'TERM',
  ]
  const safeEnv = {}
  for (const key of allowedKeys) {
    if (env[key] !== undefined) {
      safeEnv[key] = env[key]
    }
  }
  return safeEnv
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

function startPythonAgent() {
  const isDev = !app.isPackaged || process.env.ELECTRON_DEV === 'true'
  agentAuthToken = generateAuthToken()

  const safeEnv = filterSafeEnv(process.env)
  safeEnv.AGENT_AUTH_TOKEN = agentAuthToken

  if (isDev) {
    pythonProcess = spawn('python', ['agent/main.py'], {
      cwd: path.join(__dirname, '..', '..'),
      env: safeEnv,
    })
  } else {
    pythonProcess = spawn('python', ['agent/main.py'], {
      cwd: process.resourcesPath,
      env: safeEnv,
    })
  }

  pythonProcess.stderr.on('data', (data) => {
    process.stderr.write(`[Agent] ${data}`)
  })

  pythonProcess.on('error', (err) => {
    if (err.code === 'ENOENT') {
      dialog.showErrorBox('启动失败', '未检测到 Python，请先安装 Python 3.9 或更高版本。')
    } else if (!isQuiting) {
      dialog.showErrorBox('Agent 启动失败', `无法启动 Python Agent：${err.message}`)
    }
  })

  pythonProcess.on('exit', (code) => {
    if (code !== 0 && !isQuiting) {
      dialog.showErrorBox('Agent 异常退出', `Python Agent 已退出，退出码：${code}`)
    }
    pythonProcess = null
  })
}

function stopPythonAgent() {
  if (!pythonProcess) return Promise.resolve()
  return new Promise((resolve) => {
    const pid = pythonProcess.pid
    const timeout = setTimeout(() => {
      try { process.kill(pid, 'SIGKILL') } catch { /* already dead */ }
      pythonProcess = null
      resolve()
    }, 3000)

    pythonProcess.on('exit', () => {
      clearTimeout(timeout)
      pythonProcess = null
      resolve()
    })

    try { process.kill(pid, 'SIGTERM') } catch { /* already dead */ }
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

function getDataPath() {
  const home = app.getPath('home') || process.env.USERPROFILE || process.env.HOME
  return path.join(home, '.flowpartner')
}

function getWindowState() {
  const settingsPath = path.join(getDataPath(), 'settings.json')
  try {
    const data = fs.readFileSync(settingsPath, 'utf-8')
    const settings = JSON.parse(data)
    return {
      x: settings.window_x || 100,
      y: settings.window_y || 100,
      width: settings.window_width || 1200,
      height: settings.window_height || 800,
    }
  } catch {
    return { x: 100, y: 100, width: 1200, height: 800 }
  }
}

function saveWindowState(state) {
  try {
    const settingsPath = path.join(getDataPath(), 'settings.json')
    const data = fs.readFileSync(settingsPath, 'utf-8')
    const settings = JSON.parse(data)
    if (state.window_x !== undefined) settings.window_x = state.window_x
    if (state.window_y !== undefined) settings.window_y = state.window_y
    if (state.window_width !== undefined) settings.window_width = state.window_width
    if (state.window_height !== undefined) settings.window_height = state.window_height
    // 原子写入：先写临时文件再 rename，防止写入过程中进程被杀导致文件损坏
    const tmpPath = settingsPath + '.tmp'
    fs.writeFileSync(tmpPath, JSON.stringify(settings, null, 2))
    fs.renameSync(tmpPath, settingsPath)
  } catch (err) {
    console.error('Failed to save window state:', err)
  }
}

function getCloseBehavior() {
  const settingsPath = path.join(getDataPath(), 'settings.json')
  try {
    const data = fs.readFileSync(settingsPath, 'utf-8')
    const settings = JSON.parse(data)
    return {
      behavior: settings.close_behavior || 'ask',
      remembered: settings.close_remembered || false,
    }
  } catch {
    return { behavior: 'ask', remembered: false }
  }
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

async function quitAppWithCleanup() {
  if (cleanupDone) return
  cleanupDone = true
  isQuiting = true

  if (mainWindow && !mainWindow.isDestroyed()) {
    const [x, y] = mainWindow.getPosition()
    const [width, height] = mainWindow.getSize()

    try {
      const settingsPath = path.join(getDataPath(), 'settings.json')
      const data = fs.readFileSync(settingsPath, 'utf-8')
      const settings = JSON.parse(data)
      settings.window_x = x
      settings.window_y = y
      settings.window_width = width
      settings.window_height = height
      // 原子写入：先写临时文件再 rename，防止写入过程中进程被杀导致文件损坏
      const tmpPath = settingsPath + '.tmp'
      fs.writeFileSync(tmpPath, JSON.stringify(settings, null, 2))
      fs.renameSync(tmpPath, settingsPath)
    } catch (err) {
      console.error('Failed to save window state:', err)
    }
  }

  if (tray) {
    tray.destroy()
    tray = null
  }

  await stopPythonAgent()
  await stopGoProcess()

  app.quit()
}

function quitApp() {
  isQuiting = true
  app.quit()
}

function createWindow(port) {
  const state = getWindowState()

  const displays = screen.getAllDisplays()
  const isOnScreen = displays.some(d => {
    const { x, y, width, height } = d.workArea
    return state.x >= x && state.x < x + width && state.y >= y && state.y < y + height
  })

  mainWindow = new BrowserWindow({
    x: isOnScreen ? state.x : 100,
    y: isOnScreen ? state.y : 100,
    width: state.width,
    height: state.height,
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

  let moveSaveTimer = null
  let resizeSaveTimer = null
  mainWindow.on('move', () => {
    if (moveSaveTimer) clearTimeout(moveSaveTimer)
    moveSaveTimer = setTimeout(() => {
      const [x, y] = mainWindow.getPosition()
      saveWindowState({ window_x: x, window_y: y })
    }, 500)
  })

  mainWindow.on('resize', () => {
    if (resizeSaveTimer) clearTimeout(resizeSaveTimer)
    resizeSaveTimer = setTimeout(() => {
      const [width, height] = mainWindow.getSize()
      saveWindowState({ window_width: width, window_height: height })
    }, 500)
  })

  mainWindow.on('closed', () => {
    if (moveSaveTimer) clearTimeout(moveSaveTimer)
    if (resizeSaveTimer) clearTimeout(resizeSaveTimer)
    mainWindow = null
  })

  mainWindow.on('close', (event) => {
    if (!isQuiting) {
      event.preventDefault()

      const { behavior, remembered } = getCloseBehavior()

      if (remembered && behavior !== 'ask') {
        if (behavior === 'quit') {
          quitAppWithCleanup()
        } else {
          mainWindow.hide()
        }
      } else {
        mainWindow.hide()
      }
    }
  })

  const isDev = !app.isPackaged || process.env.ELECTRON_DEV === 'true'

  if (isDev) {
    mainWindow.loadURL('http://localhost:5173')
  } else {
    mainWindow.loadURL(`http://localhost:${port}`)
  }
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
      click: quitAppWithCleanup,
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

function setupPowerMonitor() {
  powerMonitor.on('suspend', () => {
    if (mainWindow) {
      mainWindow.webContents.send('system-lock')
    }
  })

  powerMonitor.on('lock-screen', () => {
    if (mainWindow) {
      mainWindow.webContents.send('system-lock')
    }
  })
}

ipcMain.handle('get-app-version', () => {
  return app.getVersion()
})

app.whenReady().then(async () => {
  try {
    createTray()
    createApplicationMenu()
    setupPowerMonitor()
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
  startPythonAgent()

  try {
    const readyPort = await waitForReady(10000)
    createWindow(readyPort)
  } catch (err) {
    dialog.showErrorBox('启动失败', '无法启动后端服务，请重启应用。')
    app.quit()
  }
})

app.on('before-quit', async () => {
  if (cleanupDone) return
  cleanupDone = true
  isQuiting = true
  if (tray) {
    tray.destroy()
    tray = null
  }
  await stopPythonAgent()
  await stopGoProcess()
})

app.on('window-all-closed', () => {
  if (process.platform !== 'darwin') {
    isQuiting = true
    app.quit()
  }
})
