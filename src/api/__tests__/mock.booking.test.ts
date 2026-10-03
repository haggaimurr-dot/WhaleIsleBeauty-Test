/** 客人端预约流程：下单、付定金、取消、改期、自动取消 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { freshMock, useMockClock } from '@/test/helpers'

let api: Awaited<ReturnType<typeof freshMock>>['api']
let simulatePaid: Awaited<ReturnType<typeof freshMock>>['simulatePaid']

beforeEach(async () => {
  useMockClock()
  ;({ api, simulatePaid } = await freshMock())
})
afterEach(() => { vi.useRealTimers() })

/** 每个用例都重新加载模块，ApiError 不是同一个类，按 code 判断 */
const code = (e: unknown) => (e as { code?: string })?.code

/** 找某位化妆师某天第一个可约时段 */
async function freeSlot(artistId: string, date: string, skip: string[] = []) {
  const s = (await api.listSlots(artistId, date, 's1')).find(x => x.available && !skip.includes(x.time))
  if (!s) throw new Error(`${artistId} ${date} 没有空时段`)
  return s.time
}

/** 第 n 个可预约日期（0 = 明天） */
const day = async (n: number) => (await api.listBookableDates())[n]

describe('可约日期和时段', () => {
  it('客人端从明天开始，共 7 天', async () => {
    const dates = await api.listBookableDates()
    expect(dates).toHaveLength(7)
    expect(dates[0]).toBe('2026-10-01')
  })

  it('同一时段不能被约两次', async () => {
    const date = await day(2)
    const time = await freeSlot('a1', date)
    await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    expect((await api.listSlots('a1', date, 's1')).find(s => s.time === time)?.available).toBe(false)
    await expect(api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })).rejects.toSatisfy(e => code(e) === 'SLOT_TAKEN')
  })
})

describe('待付定金', () => {
  it('下单后在“即将到来”里，带付款截止时间，随时可以取消', async () => {
    const date = await day(0) // 明天，不到 24 小时也能取消
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: await freeSlot('a1', date) })
    const b = (await api.listMyBookings('upcoming')).find(x => x.id === booking.id)!
    expect(b.status).toBe('pending_payment')
    expect(Date.parse(b.payDeadline!) - Date.parse(b.createdAt)).toBe(15 * 60 * 1000)
    expect(b.canCancel).toBe(true)
    const cancelled = await api.cancelBooking(b.id)
    expect(cancelled.cancelReason).toBe('customer')
  })

  it('可以继续付定金，付完变成等待确认', async () => {
    const date = await day(2)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: await freeSlot('a1', date) })
    expect((await api.resumePayment(booking.id)).booking.id).toBe(booking.id)
    await simulatePaid(booking.id)
    const b = await api.getBooking(booking.id)
    expect(b.status).toBe('pending_confirm')
    expect(b.payDeadline).toBeUndefined()
    await expect(api.resumePayment(booking.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('超过 15 分钟没付：自动取消，时段放出来，不能再付', async () => {
    const date = await day(2)
    const time = await freeSlot('a1', date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    vi.setSystemTime(Date.now() + 16 * 60 * 1000)
    const b = await api.getBooking(booking.id)
    expect(b).toMatchObject({ status: 'cancelled', cancelReason: 'pay_timeout' })
    expect((await api.listMyBookings('past')).some(x => x.id === booking.id)).toBe(true)
    expect((await api.listSlots('a1', date, 's1')).find(s => s.time === time)?.available).toBe(true)
    await expect(api.resumePayment(booking.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })
})

describe('取消和改期', () => {
  it('付过定金、不到 24 小时的预约不能在线取消', async () => {
    const date = await day(0)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: await freeSlot('a1', date) })
    await simulatePaid(booking.id)
    expect((await api.getBooking(booking.id)).canCancel).toBe(false)
    await expect(api.cancelBooking(booking.id)).rejects.toSatisfy(e => code(e) === 'CANCEL_TOO_LATE')
  })

  it('改期时自己原来的时段按可约返回，别人的仍是约满', async () => {
    const date = await day(3)
    const time = await freeSlot('a1', date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    await simulatePaid(booking.id)
    const mine = await api.listSlots('a1', date, 's1', booking.id)
    expect(mine.find(s => s.time === time)?.available).toBe(true)
    const others = await api.listSlots('a1', date, 's1')
    expect(others.find(s => s.time === time)?.available).toBe(false)
  })

  it('还没付定金的预约改期后仍是待付定金，不会绕过付款', async () => {
    const date = await day(3)
    const t1 = await freeSlot('a1', date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: t1 })
    const t2 = await freeSlot('a1', date, [t1])
    const moved = await api.rescheduleBooking(booking.id, { artistId: 'a1', date, time: t2 })
    expect(moved).toMatchObject({ time: t2, status: 'pending_payment' })
    expect(moved.payDeadline).toBe(booking.payDeadline)
  })

  it('改期后回到等待确认；改到约满的时段报 SLOT_TAKEN', async () => {
    const date = await day(3)
    const t1 = await freeSlot('a1', date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: t1 })
    await simulatePaid(booking.id)
    await api.confirmBooking(booking.id)
    const t2 = await freeSlot('a1', date, [t1])
    const moved = await api.rescheduleBooking(booking.id, { artistId: 'a1', date, time: t2 })
    expect(moved).toMatchObject({ time: t2, status: 'pending_confirm' })

    const taken = (await api.listSlots('a2', date, 's1')).find(s => !s.available)!.time
    await expect(api.rescheduleBooking(booking.id, { artistId: 'a2', date, time: taken }))
      .rejects.toSatisfy(e => code(e) === 'SLOT_TAKEN')
  })
})

describe('自动取消：到时间店里还没确认', () => {
  it('没确认的取消并记为 not_confirmed；已确认的过了结束时间算完成', async () => {
    const date = await day(0)
    const tA = await freeSlot('a1', date)
    const tB = await freeSlot('a2', date)
    const { booking: a } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time: tA })
    const { booking: b } = await api.createBooking({ serviceId: 's1', artistId: 'a2', date, time: tB })
    await simulatePaid(a.id)
    await simulatePaid(b.id)
    await api.confirmBooking(b.id)

    vi.setSystemTime(new Date(2026, 9, 1, 23, 0))
    expect(await api.getBooking(a.id)).toMatchObject({ status: 'cancelled', cancelReason: 'not_confirmed' })
    expect((await api.getBooking(b.id)).status).toBe('completed')
    expect((await api.listMyBookings('upcoming')).some(x => x.id === a.id)).toBe(false)
  })
})

describe('我的', () => {
  it('visitCount 是已完成的预约数，肤质摘要来自档案', async () => {
    const me = await api.getMe()
    expect(me.visitCount).toBe(3)
    expect(me.skinProfile).toBe('敏感肌 · 冷白皮')
  })

  it('“已完成”按时间倒序', async () => {
    const past = await api.listMyBookings('past')
    const keys = past.map(b => b.date + b.time)
    expect(keys).toEqual([...keys].sort().reverse())
  })
})
