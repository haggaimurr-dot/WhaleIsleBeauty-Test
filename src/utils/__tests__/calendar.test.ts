/** 添加到手机日历：事件内容和失败分类 */
import { afterEach, describe, expect, it, vi } from 'vitest'
import type { Booking, Shop } from '@/api'
import { addToCalendar, calendarEvent, CALENDAR_ALARM_OFFSET, classifyCalendarError } from '@/utils/calendar'

afterEach(() => vi.unstubAllGlobals())

const booking = {
  id: 'b1', serviceId: 's1', serviceName: '新娘跟妆', artistId: 'a1', artistName: '小鲸',
  date: '2026-10-08', time: '14:00', durationMin: 90, price: 128000, deposit: 20000,
  status: 'confirmed', createdAt: '', canCancel: true,
} as Booking
const shop = { name: '鲸屿化妆室', address: '某路 1 号', phone: '13800000000', openHours: '', latitude: 0, longitude: 0 } as Shop

describe('calendar', () => {
  it('按本地时间算开始、结束，带地址和门店电话，开始前 1 小时提醒', () => {
    const e = calendarEvent(booking, shop)
    const start = new Date(2026, 9, 8, 14, 0).getTime() / 1000
    expect(e.startTime).toBe(start)
    expect(e.endTime).toBe(String(start + 90 * 60))
    expect(e.title).toBe('新娘跟妆 · 鲸屿化妆室')
    expect(e.location).toBe('某路 1 号')
    expect(e.description).toContain('13800000000')
    expect(e.alarmOffset).toBe(CALENDAR_ALARM_OFFSET)
  })

  it('门店信息没拿到时也能加', () => {
    const e = calendarEvent(booking)
    expect(e.title).toBe('新娘跟妆')
    expect(e.location).toBeUndefined()
    expect(e.description).not.toContain('门店电话')
  })

  it('失败分类：关掉确认框、没权限、其他', () => {
    expect(classifyCalendarError('addPhoneCalendar:fail cancel')).toBe('cancelled')
    expect(classifyCalendarError('addPhoneCalendar:fail auth deny')).toBe('denied')
    expect(classifyCalendarError('addPhoneCalendar:fail auth denied')).toBe('denied')
    expect(classifyCalendarError('addPhoneCalendar:fail api scope is not declared in the privacy agreement')).toBe('failed')
    expect(classifyCalendarError()).toBe('failed')
  })

  it('关掉确认框不提示；没权限引导去设置', async () => {
    const toasts: string[] = []
    const modals: string[] = []
    let errMsg = 'addPhoneCalendar:fail cancel'
    vi.stubGlobal('uni', {
      addPhoneCalendar: (o: { fail: (e: { errMsg: string }) => void }) => o.fail({ errMsg }),
      showToast: (o: { title: string }) => toasts.push(o.title),
      showModal: (o: { title: string }) => modals.push(o.title),
    })
    expect(await addToCalendar(booking, shop)).toBe('cancelled')
    expect(toasts).toEqual([])
    expect(modals).toEqual([])
    errMsg = 'addPhoneCalendar:fail auth deny'
    expect(await addToCalendar(booking, shop)).toBe('denied')
    expect(modals).toEqual(['没有日历权限'])
  })
})
