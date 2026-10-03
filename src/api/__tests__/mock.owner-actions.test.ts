/** 店主替客人取消、改期，标记客人没来；以及已确认的预约到点自动完成 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { DaySchedule, ScheduleCell } from '@/api/types'
import { freshMock, useMockClock } from '@/test/helpers'

let api: Awaited<ReturnType<typeof freshMock>>['api']
let simulatePaid: Awaited<ReturnType<typeof freshMock>>['simulatePaid']
let ownerNotices: Awaited<ReturnType<typeof freshMock>>['ownerNotices']

beforeEach(async () => {
  useMockClock() // 2026-09-30 10:00
  ;({ api, simulatePaid, ownerNotices } = await freshMock())
})
afterEach(() => { vi.useRealTimers() })

const code = (e: unknown) => (e as { code?: string })?.code
const at = (date: string, time: string) => new Date(`${date}T${time}:00`)
const cellAt = (s: DaySchedule, artistId: string, time: string) =>
  s.cells.find(c => c.artistId === artistId && c.time === time) as ScheduleCell

/** 某天某位化妆师空闲的格子 */
async function freeCells(date: string, artistId = 'a1') {
  return (await api.getDaySchedule(date)).cells.filter(x => x.artistId === artistId && x.state === 'free').map(c => c.time)
}

/** 下单并付定金；confirm 为 true 时店主再确认 */
async function paidBooking(date: string, time: string, confirm = false, artistId = 'a1') {
  const { booking } = await api.createBooking({ serviceId: 's1', artistId, date, time })
  await simulatePaid(booking.id)
  if (confirm) await api.confirmBooking(booking.id)
  return booking
}

describe('店主替客人取消', () => {
  it('不受 24 小时限制，定金退回，时段放出来，不给店主发提醒', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    await api.addOwnerNotify(3)
    const sent = ownerNotices().length
    vi.setSystemTime(at(date, '08:00')) // 不到 24 小时，客人自己已经取消不了
    await expect(api.cancelBooking(b.id)).rejects.toSatisfy(e => code(e) === 'CANCEL_TOO_LATE')

    const out = await api.ownerCancelBooking(b.id)
    expect(out).toMatchObject({ status: 'cancelled', cancelReason: 'customer' })
    expect(cellAt(await api.getDaySchedule(date), 'a1', time).state).toBe('free')
    expect(ownerNotices()).toHaveLength(sent)
    await expect(api.ownerCancelBooking(b.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('还没付定金的也能取消', async () => {
    const date = '2026-10-02'
    const [time] = await freeCells(date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    expect(await api.ownerCancelBooking(booking.id)).toMatchObject({ status: 'cancelled' })
  })

  it('开始时间过了不能再取消', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    vi.setSystemTime(at(date, time))
    await expect(api.ownerCancelBooking(b.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })
})

describe('店主替客人改期', () => {
  it('不受 24 小时限制；付过定金的改完直接已确认，原来的时段放出来', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time)
    vi.setSystemTime(at(date, '08:00'))
    const to = (await freeCells('2026-10-03', 'a2'))[0]
    const out = await api.ownerRescheduleBooking(b.id, { artistId: 'a2', date: '2026-10-03', time: to })
    expect(out).toMatchObject({ status: 'confirmed', artistId: 'a2', artistName: '安安', date: '2026-10-03', time: to })
    expect(cellAt(await api.getDaySchedule(date), 'a1', time).state).toBe('free')
    expect(cellAt(await api.getDaySchedule('2026-10-03'), 'a2', to)).toMatchObject({ state: 'booked', booking: { id: b.id } })
  })

  it('可以改到今天还没过的时段', async () => {
    const b = await paidBooking('2026-10-02', (await freeCells('2026-10-02'))[0], true)
    const today = (await freeCells('2026-09-30', 'a3'))[0]
    expect(await api.ownerRescheduleBooking(b.id, { artistId: 'a3', date: '2026-09-30', time: today })).toMatchObject({ date: '2026-09-30' })
  })

  it('还没付定金的仍是待付定金', async () => {
    const date = '2026-10-02'
    const [time, to] = await freeCells(date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    expect(await api.ownerRescheduleBooking(booking.id, { artistId: 'a1', date, time: to })).toMatchObject({ status: 'pending_payment', time: to })
  })

  it('改到有预约、休息或已经过去的时段返回 SLOT_TAKEN', async () => {
    const date = '2026-10-02'
    const [time, taken, blocked] = await freeCells(date)
    const b = await paidBooking(date, time)
    await paidBooking(date, taken)
    await api.blockSlot('a1', date, blocked)
    for (const t of [taken, blocked]) {
      await expect(api.ownerRescheduleBooking(b.id, { artistId: 'a1', date, time: t })).rejects.toSatisfy(e => code(e) === 'SLOT_TAKEN')
    }
    await expect(api.ownerRescheduleBooking(b.id, { artistId: 'a1', date: '2026-09-30', time: '09:00' }))
      .rejects.toSatisfy(e => code(e) === 'SLOT_TAKEN')
  })

  it('开始时间过了不能再改', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    vi.setSystemTime(at(date, time))
    await expect(api.ownerRescheduleBooking(b.id, { artistId: 'a1', date: '2026-10-03', time: '19:30' }))
      .rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })
})

describe('排班里的操作开关', () => {
  it('还没开始的进行中预约能改；别的客人的演示格子不能', async () => {
    const date = '2026-10-02'
    const [time] = await freeCells(date)
    await paidBooking(date, time)
    const s = await api.getDaySchedule(date)
    expect(cellAt(s, 'a1', time).booking).toMatchObject({ canChange: true, canMarkNoShow: false })
    const others = s.cells.filter(c => c.booking?.id.startsWith('other-'))
    expect(others.length).toBeGreaterThan(0)
    expect(others.every(c => !c.booking?.canChange && !c.booking?.canMarkNoShow)).toBe(true)
  })
})

describe('客人没来', () => {
  it('开始前不能标记；开始后当天可以，定金退回，格子还在但不计入当天预约', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    await expect(api.markNoShow(b.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')

    vi.setSystemTime(at(date, time))
    vi.advanceTimersByTime(10 * 60 * 1000)
    const before = await api.getDaySchedule(date)
    expect(cellAt(before, 'a1', time).booking).toMatchObject({ status: 'confirmed', canMarkNoShow: true, canChange: false })

    expect(await api.markNoShow(b.id)).toMatchObject({ status: 'cancelled', cancelReason: 'no_show' })
    const after = await api.getDaySchedule(date)
    expect(cellAt(after, 'a1', time)).toMatchObject({ state: 'booked', booking: { status: 'cancelled', canMarkNoShow: false } })
    expect(after.stats.total).toBe(before.stats.total - 1)
    await expect(api.markNoShow(b.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('过了结束时间已经自动完成，当天还能标记，到店次数跟着减掉', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    const visits = (await api.getMe()).visitCount
    vi.setSystemTime(at(date, '23:00'))
    expect(await api.getBooking(b.id)).toMatchObject({ status: 'completed' })
    expect((await api.getMe()).visitCount).toBe(visits + 1)
    expect(cellAt(await api.getDaySchedule(date), 'a1', time)).toMatchObject({ state: 'booked', booking: { status: 'completed', canMarkNoShow: true } })

    await api.markNoShow(b.id)
    expect((await api.getMe()).visitCount).toBe(visits)
    expect((await api.listMyBookings('past')).find(x => x.id === b.id)).toMatchObject({ cancelReason: 'no_show' })
  })

  it('第二天就不能再标记了', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    vi.setSystemTime(at('2026-10-02', '09:00'))
    expect(cellAt(await api.getDaySchedule(date), 'a1', time).booking).toMatchObject({ canMarkNoShow: false })
    await expect(api.markNoShow(b.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })
})

describe('经营统计里的没来', () => {
  it('算在取消里，单独记一个数，定金退回不算收入', async () => {
    const date = '2026-10-01'
    const [time] = await freeCells(date)
    const b = await paidBooking(date, time, true)
    vi.setSystemTime(at(date, '23:00'))
    const oct = async () => (await api.listMonthStats()).find(m => m.month === '2026-10')!
    const before = await oct()
    await api.markNoShow(b.id)
    const after = await oct()
    expect(after.bookings).toBe(before.bookings - 1)
    expect(after.cancelled).toBe(before.cancelled + 1)
    expect(after.noShow).toBe(before.noShow + 1)
    expect(after.deposit).toBe(before.deposit - b.deposit)
  })
})
