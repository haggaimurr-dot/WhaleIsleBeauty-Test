/** 店主排班：日期、格子状态、统计、确认、休息、详情字段；以及肤质档案 */
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
const cellAt = (s: DaySchedule, artistId: string, time: string) =>
  s.cells.find(c => c.artistId === artistId && c.time === time) as ScheduleCell

/** 某天某位化妆师第一个空闲格子 */
async function freeCell(date: string, artistId = 'a1') {
  const c = (await api.getDaySchedule(date)).cells.find(x => x.artistId === artistId && x.state === 'free')
  if (!c) throw new Error('没有空闲格子')
  return c.time
}

describe('排班日期和格子', () => {
  it('排班从今天开始，客人端从明天开始', async () => {
    const owner = await api.listScheduleDates()
    expect(owner[0]).toBe('2026-09-30')
    expect(owner[1]).toBe((await api.listBookableDates())[0])
  })

  it('今天已经过去的空闲格子是 past，不能设休息；统计不算它们', async () => {
    const s = await api.getDaySchedule('2026-09-30')
    const past = s.cells.filter(c => c.time <= '10:00' && !c.booking)
    expect(past.length).toBeGreaterThan(0)
    expect(past.every(c => c.state === 'past')).toBe(true)
    expect(s.stats.free).toBe(s.cells.filter(c => c.state === 'free').length)
    await expect(api.blockSlot(past[0].artistId, '2026-09-30', past[0].time)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('空闲格子可以设休息，再恢复可约', async () => {
    const date = '2026-10-02'
    const time = await freeCell(date)
    await api.blockSlot('a1', date, time)
    expect(cellAt(await api.getDaySchedule(date), 'a1', time).state).toBe('blocked')
    expect((await api.listSlots('a1', date, 's1')).find(s => s.time === time)?.available).toBe(false)
    await api.unblockSlot('a1', date, time)
    expect(cellAt(await api.getDaySchedule(date), 'a1', time).state).toBe('free')
  })
})

describe('客人的预约在排班里', () => {
  it('待付定金：显示为已约，不能确认，不计入当天预约，不能设休息', async () => {
    const date = '2026-10-02'
    const before = await api.getDaySchedule(date)
    const time = await freeCell(date)
    await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    const after = await api.getDaySchedule(date)
    const c = cellAt(after, 'a1', time)
    expect(c.state).toBe('booked')
    expect(c.booking).toMatchObject({ status: 'pending_payment', canConfirm: false })
    expect(after.stats.total).toBe(before.stats.total)
    expect(after.stats.free).toBe(before.stats.free - 1)
    await expect(api.blockSlot('a1', date, time)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('付完定金：待确认，可以确认；确认后变成已约', async () => {
    const date = '2026-10-02'
    const time = await freeCell(date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    await simulatePaid(booking.id)
    let s = await api.getDaySchedule(date)
    expect(cellAt(s, 'a1', time)).toMatchObject({ state: 'pending', booking: { canConfirm: true } })
    expect(s.stats.pending - s.stats.stale).toBeGreaterThanOrEqual(1)
    await api.confirmBooking(booking.id)
    s = await api.getDaySchedule(date)
    expect(cellAt(s, 'a1', time).state).toBe('booked')
    await expect(api.confirmBooking(booking.id)).rejects.toSatisfy(e => code(e) === 'INVALID_STATE')
  })

  it('详情字段：时长、场合、这次肤质、档案；过敏合并进备注', async () => {
    const date = '2026-10-02'
    const time = await freeCell(date)
    const { booking } = await api.createBooking({
      serviceId: 's1', artistId: 'a1', date, time, occasion: 'photo', skinType: 'dry', note: '想要自然一点',
    })
    await simulatePaid(booking.id)
    const brief = cellAt(await api.getDaySchedule(date), 'a1', time).booking!
    expect(brief).toMatchObject({
      durationMin: 90, occasion: 'photo', skinType: 'dry',
      alert: '敏感肌', // 这次选了干皮，但档案里是敏感肌
      note: '想要自然一点；过敏：对酒精过敏',
      profile: { skinType: 'sensitive', tone: 'cool_fair', allergies: '对酒精过敏' },
    })
  })
})

describe('肤质档案', () => {
  it('保存会整份替换；空字符串清空；摘要跟着变', async () => {
    const saved = await api.updateSkinProfile({ allergies: '  ', note: '单眼皮' })
    expect(saved.allergies).toBeUndefined()
    expect(saved.skinType).toBeUndefined()
    expect(saved.note).toBe('单眼皮')
    expect(saved.updatedAt).toBeTruthy()
    expect((await api.getMe()).skinProfile).toBe('已填写')

    await api.updateSkinProfile({ skinType: 'oily', tone: 'unsure' })
    expect((await api.getMe()).skinProfile).toBe('油皮')

    await api.updateSkinProfile({})
    expect((await api.getMe()).skinProfile).toBeUndefined()
  })

  it('清空档案后，排班里不再提示过敏', async () => {
    await api.updateSkinProfile({})
    const date = '2026-10-02'
    const time = await freeCell(date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    await simulatePaid(booking.id)
    const brief = cellAt(await api.getDaySchedule(date), 'a1', time).booking!
    expect(brief.alert).toBeUndefined()
    expect(brief.note).toBeUndefined()
    expect(brief.profile).toBeUndefined()
  })
})

describe('通知店主', () => {
  /** 约明天 a1 的一个空闲时段；pay 为 true 时顺便付定金 */
  async function book(pay = true) {
    const date = '2026-10-02'
    const time = await freeCell(date)
    const { booking } = await api.createBooking({ serviceId: 's1', artistId: 'a1', date, time })
    if (pay) await simulatePaid(booking.id)
    return booking
  }

  it('额度从 0 开始，每次同意加上，一次最多加 5，不能是 0 或负数', async () => {
    expect((await api.getOwnerNotify()).quota).toBe(0)
    expect((await api.addOwnerNotify(1)).quota).toBe(1)
    expect((await api.addOwnerNotify(5)).quota).toBe(6)
    for (const n of [0, -1, 6, 1.5]) {
      await expect(api.addOwnerNotify(n)).rejects.toSatisfy(e => code(e) === 'UNKNOWN')
    }
    expect((await api.getOwnerNotify()).quota).toBe(6)
  })

  it('付完定金、改期、取消各发一条，每条用掉一次额度', async () => {
    await api.addOwnerNotify(5)
    const b = await book()
    expect(ownerNotices()).toEqual([{ kind: 'new', bookingId: b.id }])
    const time = await freeCell('2026-10-03')
    await api.rescheduleBooking(b.id, { artistId: 'a1', date: '2026-10-03', time })
    await api.cancelBooking(b.id)
    expect(ownerNotices().map(n => n.kind)).toEqual(['new', 'rescheduled', 'cancelled'])
    expect((await api.getOwnerNotify()).quota).toBe(2)
  })

  it('还没付定金的预约改期、取消、超时都不打扰店主', async () => {
    await api.addOwnerNotify(5)
    const b = await book(false)
    const time = await freeCell('2026-10-03')
    await api.rescheduleBooking(b.id, { artistId: 'a1', date: '2026-10-03', time })
    await api.cancelBooking(b.id)
    expect(ownerNotices()).toEqual([])
    expect((await api.getOwnerNotify()).quota).toBe(5)
  })

  it('额度用完就不发，也不会变成负数', async () => {
    await api.addOwnerNotify(1)
    await book()
    await book()
    expect(ownerNotices()).toHaveLength(1)
    expect((await api.getOwnerNotify()).quota).toBe(0)
  })

})
