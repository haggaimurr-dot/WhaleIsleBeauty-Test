/**
 * 把预约加到手机系统日历（wx.addPhoneCalendar，基础库 2.15.0）。
 *
 * 微信会先弹系统确认框，客人点了才写进日历。写进去的日程不会跟着改期、取消更新，
 * 所以事件说明和成功提示里都提醒一句。
 *
 * 这个接口属于隐私接口：小程序后台「用户隐私保护指引」里要勾上“日历”，否则一调就失败。
 */
import type { Booking, Shop } from '@/api'
import { parse } from './date'

/** 开始前 1 小时提醒。前一天的提醒由订阅消息负责，这里不重复 */
export const CALENDAR_ALARM_OFFSET = 3600

export function calendarEvent(b: Booking, shop?: Shop): UniNamespace.AddPhoneCalendarOption {
  const start = Math.floor(parse(b.date, b.time).getTime() / 1000)
  const description = [
    `化妆师 ${b.artistName}，约 ${b.durationMin} 分钟。素颜过来就好。`,
    shop ? `门店电话 ${shop.phone}` : '',
    '改期或取消请在小程序「我的」里操作，这条日程不会自动跟着变。',
  ].filter(Boolean).join('\n')
  return {
    title: shop ? `${b.serviceName} · ${shop.name}` : b.serviceName,
    startTime: start,
    // 微信的类型定义里 endTime 是字符串
    endTime: String(start + b.durationMin * 60),
    location: shop?.address,
    description,
    alarm: true,
    alarmOffset: CALENDAR_ALARM_OFFSET,
  }
}

export type CalendarResult = 'added' | 'cancelled' | 'denied' | 'failed'

export function classifyCalendarError(errMsg = ''): Exclude<CalendarResult, 'added'> {
  if (/cancel/i.test(errMsg)) return 'cancelled'
  if (/auth ?den|authoriz/i.test(errMsg)) return 'denied'
  return 'failed'
}

/** 加到日历并给出提示。客人自己关掉确认框时不提示 */
export function addToCalendar(b: Booking, shop?: Shop): Promise<CalendarResult> {
  return new Promise(resolve => {
    uni.addPhoneCalendar({
      ...calendarEvent(b, shop),
      success: () => {
        uni.showToast({ title: '已加到手机日历，改期的话记得也改一下', icon: 'none', duration: 2500 })
        resolve('added')
      },
      fail: (e: { errMsg?: string }) => {
        const r = classifyCalendarError(e?.errMsg)
        if (r === 'denied') {
          uni.showModal({
            title: '没有日历权限',
            content: '在设置里允许“添加日历事件”，就能把预约加到手机日历了。',
            confirmText: '去设置',
            cancelText: '先不用',
            confirmColor: '#6E5446',
            success: ({ confirm }) => { if (confirm) uni.openSetting({}) },
          })
        } else if (r === 'failed') {
          uni.showToast({ title: '这台手机暂时加不了日历，记一下时间就好', icon: 'none', duration: 2500 })
        }
        resolve(r)
      },
    })
  })
}
