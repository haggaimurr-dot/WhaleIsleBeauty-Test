/** 店主经营统计：月份范围、只算付过定金的、取消率、回头客 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { MonthStats } from '@/api/types'
import { freshMock, useMockClock } from '@/test/helpers'

let api: Awaited<ReturnType<typeof freshMock>>['api']
let simulatePaid: Awaited<ReturnType<typeof freshMock>>['simulatePaid']

beforeEach(async () => {
  // 月中：可预约的日期（9/11–9/17）都在本月，方便看本月的变化
  useMockClock(new Date(2026, 8, 10, 10, 0))
  ;({ api, simulatePaid } = await freshMock())
})
afterEach(() => { vi.useRealTimers() })

const sept = async () => (await api.listMonthStats()).find(m => m.month === '2026-09') as MonthStats

/** 某天 a1 第一个能约的时间，下一个预约 */
async function book(date: string, serviceId = 's1') {
  const slot = (await api.listSlots('a1', date, serviceId)).find(s => s.available)
  if (!slot) throw new Error('没有能约的时间')
  return (await api.createBooking({ serviceId, artistId: 'a1', date, time: slot.time })).booking
}

describe('月份', () => {
  it('最近 6 个月，本月在前', async () => {
    const list = await api.listMonthStats()
    expect(list.map(m => m.month)).toEqual(['2026-09', '2026-08', '2026-07', '2026-06', '2026-05', '2026-04'])
  })

  it('各项不会是负数，回头客不超过客人数', async () => {
    for (const m of await api.listMonthStats()) {
      expect(m.bookings).toBeGreaterThanOrEqual(0)
      expect(m.cancelled).toBeGreaterThanOrEqual(0)
      expect(m.deposit).toBeGreaterThanOrEqual(0)
      expect(m.returning).toBeLessThanOrEqual(m.customers)
      expect(m.customers).toBeLessThanOrEqual(m.bookings)
    }
  })
})

describe('只算付过定金的', () => {
  it('没付定金不算；付了算预约数和定金', async () => {
    const before = await sept()
    const b = await book('2026-09-12')
    expect(await sept()).toEqual(before)
    await simulatePaid(b.id)
    const after = await sept()
    expect(after.bookings).toBe(before.bookings + 1)
    expect(after.deposit).toBe(before.deposit + b.deposit)
    expect(after.cancelled).toBe(before.cancelled)
  })

  it('付过定金后取消：从预约数挪到取消，定金退回不算收入', async () => {
    const before = await sept()
    const b = await book('2026-09-14')
    await simulatePaid(b.id)
    await api.cancelBooking(b.id)
    const after = await sept()
    expect(after.bookings).toBe(before.bookings)
    expect(after.deposit).toBe(before.deposit)
    expect(after.cancelled).toBe(before.cancelled + 1)
  })

  it('没付就取消、付款超时都不算取消', async () => {
    const before = await sept()
    const a = await book('2026-09-13')
    await api.cancelBooking(a.id)
    await book('2026-09-15')
    vi.advanceTimersByTime(16 * 60 * 1000)
    expect(await sept()).toEqual(before)
  })

  it('改期到别的月份，跟着到店日期走', async () => {
    useMockClock(new Date(2026, 8, 28, 10, 0)) // 可约 9/29–10/5，跨月
    ;({ api, simulatePaid } = await freshMock())
    const before = await sept()
    const b = await book('2026-09-30')
    await simulatePaid(b.id)
    expect((await sept()).bookings).toBe(before.bookings + 1)
    const slot = (await api.listSlots('a1', '2026-10-02', 's1', b.id)).find(s => s.available)!
    await api.rescheduleBooking(b.id, { artistId: 'a1', date: '2026-10-02', time: slot.time })
    expect((await sept()).bookings).toBe(before.bookings)
  })
})

describe('回头客', () => {
  it('以前到店完成过的客人这个月再来，算回头客', async () => {
    // 演示用户 8 月来过（已完成），9 月还没有预约
    const before = await sept()
    const b = await book('2026-09-16')
    await simulatePaid(b.id)
    const after = await sept()
    expect(after.customers).toBe(before.customers + 1)
    expect(after.returning).toBe(before.returning + 1)
  })

  it('同一位客人一个月约两次只算一位', async () => {
    const before = await sept()
    for (const date of ['2026-09-12', '2026-09-16']) await simulatePaid((await book(date)).id)
    const after = await sept()
    expect(after.bookings).toBe(before.bookings + 2)
    expect(after.customers).toBe(before.customers + 1)
  })
})

