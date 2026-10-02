/**
 * 测试用的小工具。mock 每次调用都有 250–500ms 的假延迟，测试里用假时钟快进，不真的等。
 */
import { vi } from 'vitest'

/** 演示数据里的“现在”：2026-09-30 周三 10:00（本地时间） */
export const NOW = new Date(2026, 8, 30, 10, 0)

export function useMockClock(at: Date = NOW) {
  vi.useFakeTimers()
  vi.setSystemTime(at)
}

/** 等一个 mock 调用完成：一边推进假时钟一边等 */
export async function settle<T>(p: Promise<T>): Promise<T> {
  let done = false
  p.then(() => { done = true }, () => { done = true })
  while (!done) await vi.advanceTimersByTimeAsync(100)
  return p
}

/** 每个用例拿一份全新的 mock（模块里有内存状态） */
export async function freshMock() {
  vi.resetModules()
  const m = await import('@/api/mock')
  // 把 api 的每个方法包一层 settle，用例里直接 await
  const api = new Proxy(m.mockApi, {
    get: (t, k: keyof typeof m.mockApi) => (...args: unknown[]) => settle((t[k] as (...a: unknown[]) => Promise<unknown>)(...args)),
  }) as typeof m.mockApi
  const simulatePaid = (id: string) => settle(m.__simulatePaid(id))
  return { api, simulatePaid, ownerNotices: m.__ownerNotices }
}

/** 本地日期 → 'YYYY-MM-DD' */
export const ymd = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
