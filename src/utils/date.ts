import type { DateStr, TimeStr, Timestamp } from '../api/types'

const WEEK = ['日', '一', '二', '三', '四', '五', '六']
const pad = (n: number) => String(n).padStart(2, '0')

export function toDateStr(d: Date): DateStr {
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}

/** iOS 不支持 new Date('YYYY-MM-DD HH:mm')，统一用这个解析 */
export function parse(date: DateStr, time: TimeStr = '00:00'): Date {
  const [y, m, d] = date.split('-').map(Number)
  const [hh, mm] = time.split(':').map(Number)
  return new Date(y, m - 1, d, hh, mm)
}

export const todayStr = () => toDateStr(new Date())
export const nowTimeStr = () => { const d = new Date(); return `${pad(d.getHours())}:${pad(d.getMinutes())}` }
export const toTimestamp = (d: Date): Timestamp => d.toISOString()

/** 从明天开始的 n 天 */
export function upcomingDates(n: number): DateStr[] {
  const t = new Date()
  return Array.from({ length: n }, (_, i) => toDateStr(new Date(t.getFullYear(), t.getMonth(), t.getDate() + 1 + i)))
}

/** '9月30日 周三' */
export function formatDateCN(date: DateStr): string {
  const d = parse(date)
  return `${d.getMonth() + 1}月${d.getDate()}日 周${WEEK[d.getDay()]}`
}

/** '8月16日'，用于历史记录 */
export function formatMonthDay(date: DateStr): string {
  const d = parse(date)
  return `${d.getMonth() + 1}月${d.getDate()}日`
}

/** 日期条上的短标签：'明天' 或 '周三' */
export function dayLabel(date: DateStr): string {
  return date === upcomingDates(1)[0] ? '明天' : `周${WEEK[parse(date).getDay()]}`
}

export const dayOfMonth = (date: DateStr) => parse(date).getDate()

export function hoursUntil(date: DateStr, time: TimeStr): number {
  return (parse(date, time).getTime() - Date.now()) / 3_600_000
}
