/**
 * 测试 cleanupDone 互斥模式
 *
 * 验证 quitAppWithCleanup 和 before-quit 之间的竞态条件修复。
 * cleanupDone 标志确保清理逻辑只执行一次。
 */
import { describe, it, expect, vi, beforeEach } from 'vitest'

// 模拟 cleanupDone 互斥模式
// 这是从 frontend/electron/main.cjs 中提取的模式
function createCleanupGuard() {
  let cleanupDone = false

  async function guardedCleanup(cleanupFn: () => Promise<void>) {
    if (cleanupDone) return
    cleanupDone = true
    await cleanupFn()
  }

  function reset() {
    cleanupDone = false
  }

  return { guardedCleanup, reset, isDone: () => cleanupDone }
}

describe('cleanupDone 互斥模式', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  it('cleanup 只执行一次，即使被调用两次', async () => {
    const { guardedCleanup } = createCleanupGuard()
    const cleanupFn = vi.fn().mockResolvedValue(undefined)

    // 同时调用两次
    const promise1 = guardedCleanup(cleanupFn)
    const promise2 = guardedCleanup(cleanupFn)

    await Promise.all([promise1, promise2])

    // cleanup 函数只应被调用一次
    expect(cleanupFn).toHaveBeenCalledTimes(1)
  })

  it('cleanup 只执行一次，即使被快速连续调用多次', async () => {
    const { guardedCleanup } = createCleanupGuard()
    const cleanupFn = vi.fn().mockResolvedValue(undefined)

    // 快速连续调用 5 次
    const promises = []
    for (let i = 0; i < 5; i++) {
      promises.push(guardedCleanup(cleanupFn))
    }

    await Promise.all(promises)

    // cleanup 函数只应被调用一次
    expect(cleanupFn).toHaveBeenCalledTimes(1)
  })

  it('reset 后可以再次执行 cleanup', async () => {
    const { guardedCleanup, reset } = createCleanupGuard()
    const cleanupFn = vi.fn().mockResolvedValue(undefined)

    // 第一次调用
    await guardedCleanup(cleanupFn)
    expect(cleanupFn).toHaveBeenCalledTimes(1)

    // Reset
    reset()

    // 第二次调用
    await guardedCleanup(cleanupFn)
    expect(cleanupFn).toHaveBeenCalledTimes(2)
  })

  it('isDone 返回正确的状态', async () => {
    const { guardedCleanup, isDone } = createCleanupGuard()
    const cleanupFn = vi.fn().mockResolvedValue(undefined)

    expect(isDone()).toBe(false)

    await guardedCleanup(cleanupFn)

    expect(isDone()).toBe(true)
  })

  it('cleanup 函数抛出异常时，cleanupDone 仍为 true（防止重试）', async () => {
    const { guardedCleanup, isDone } = createCleanupGuard()
    const cleanupFn = vi.fn().mockRejectedValue(new Error('cleanup failed'))

    await expect(guardedCleanup(cleanupFn)).rejects.toThrow('cleanup failed')

    // 即使失败，cleanupDone 也应为 true
    expect(isDone()).toBe(true)
  })

  it('模拟 quitAppWithCleanup 和 before-quit 同时触发', async () => {
    const { guardedCleanup } = createCleanupGuard()
    const stopPythonAgent = vi.fn().mockResolvedValue(undefined)
    const stopGoProcess = vi.fn().mockResolvedValue(undefined)
    const destroyTray = vi.fn()

    async function quitAppWithCleanup() {
      await guardedCleanup(async () => {
        destroyTray()
        await stopPythonAgent()
        await stopGoProcess()
      })
    }

    async function onBeforeQuit() {
      await guardedCleanup(async () => {
        destroyTray()
        await stopPythonAgent()
        await stopGoProcess()
      })
    }

    // 模拟两个事件同时触发
    await Promise.all([quitAppWithCleanup(), onBeforeQuit()])

    // 所有清理函数只应被调用一次
    expect(destroyTray).toHaveBeenCalledTimes(1)
    expect(stopPythonAgent).toHaveBeenCalledTimes(1)
    expect(stopGoProcess).toHaveBeenCalledTimes(1)
  })

  it('模拟 cleanup 过程中另一个调用等待完成', async () => {
    const { guardedCleanup } = createCleanupGuard()
    let resolveCleanup: () => void
    const cleanupFn = vi.fn().mockImplementation(() => new Promise<void>((resolve) => {
      resolveCleanup = resolve
    }))

    // 启动第一个 cleanup
    const promise1 = guardedCleanup(cleanupFn)

    // 立即启动第二个 cleanup（应该被忽略）
    const promise2 = guardedCleanup(cleanupFn)

    // 此时 cleanup 函数只被调用了一次
    expect(cleanupFn).toHaveBeenCalledTimes(1)

    // 完成 cleanup
    resolveCleanup!()
    await Promise.all([promise1, promise2])

    // 仍然只被调用一次
    expect(cleanupFn).toHaveBeenCalledTimes(1)
  })
})
