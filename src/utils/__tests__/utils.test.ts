/** 工具函数：日期、金额、分享、订阅消息 */
import { afterEach, describe, expect, it, vi } from 'vitest'
import { addMinutes, datesFromToday, dayLabel, formatClock, formatDateCN, upcomingDates } from '@/utils/date'
import { centsToYuanInput, formatPrice, parseYuan } from '@/utils/money'
import { shareImage, sharePath } from '@/utils/share'
import { requestSubscribe, SUBSCRIBE_TEMPLATES } from '@/utils/subscribe'
import { useMockClock } from '@/test/helpers'

afterEach(() => { vi.useRealTimers(); vi.unstubAllGlobals() })

describe('date', () => {
  it('客人端从明天开始，排班从今天开始；标签是今天 / 明天 / 周几', () => {
    useMockClock() // 2026-09-30 周三
    expect(upcomingDates(2)).toEqual(['2026-10-01', '2026-10-02'])
    expect(datesFromToday(2)).toEqual(['2026-09-30', '2026-10-01'])
    expect(['2026-09-30', '2026-10-01', '2026-10-02'].map(dayLabel)).toEqual(['今天', '明天', '周五'])
  })

  it('跨月、跨年也对', () => {
    useMockClock(new Date(2026, 11, 31, 9, 0))
    expect(upcomingDates(2)).toEqual(['2027-01-01', '2027-01-02'])
  })

  it('格式化', () => {
    expect(formatDateCN('2026-10-03')).toBe('10月3日 周六')
    expect(addMinutes('09:00', 90)).toBe('10:30')
    expect(addMinutes('19:30', 45)).toBe('20:15')
    expect(formatClock(new Date(2026, 8, 30, 18, 5).toISOString())).toBe('18:05')
  })
})

describe('money', () => {
  it('分转元，整数不带小数，priceFrom 加“起”', () => {
    expect(formatPrice(29800)).toBe('¥298')
    expect(formatPrice(12850)).toBe('¥128.5')
    expect(formatPrice(12805)).toBe('¥128.05')
    expect(formatPrice(128000, { from: true })).toBe('¥1280 起')
  })

  it('店主填的元转分：最多两位小数，认不出的返回 NaN', () => {
    expect(parseYuan('298')).toBe(29800)
    expect(parseYuan(' 128.5 ')).toBe(12850)
    expect(parseYuan('0.1')).toBe(10)
    expect(parseYuan('19.99')).toBe(1999) // 不能有浮点误差
    for (const bad of ['', '¥298', '1.234', 'abc', '-5', '.5']) expect(parseYuan(bad)).toBeNaN()
  })

  it('分转回输入框里的元', () => {
    expect(centsToYuanInput(29800)).toBe('298')
    expect(centsToYuanInput(12850)).toBe('128.5')
    expect(centsToYuanInput(1999)).toBe('19.99')
  })
})

describe('share', () => {
  it('路径跳过空参数并编码', () => {
    expect(sharePath('/pages/works/index', { category: 'bridal', workId: 'w3' })).toBe('/pages/works/index?category=bridal&workId=w3')
    expect(sharePath('/pages/works/index', { category: undefined, workId: '' })).toBe('/pages/works/index')
    expect(sharePath('/p', { q: 'a b' })).toBe('/p?q=a%20b')
  })

  it('占位图不作封面', () => {
    expect(shareImage('placeholder:g2')).toBeUndefined()
    expect(shareImage('https://x/a.jpg')).toBe('https://x/a.jpg')
  })
})

describe('subscribe', () => {
  it('模板 ID 为空时不调微信接口；填了几个就订几个', async () => {
    const calls: string[][] = []
    vi.stubGlobal('uni', {
      requestSubscribeMessage: (o: { tmplIds: string[]; complete: () => void }) => { calls.push(o.tmplIds); setTimeout(o.complete, 1) },
    })
    const saved = { ...SUBSCRIBE_TEMPLATES }
    try {
      Object.assign(SUBSCRIBE_TEMPLATES, { confirmed: '', reminder: '' })
      await requestSubscribe(['confirmed', 'reminder'])
      expect(calls).toHaveLength(0)
      SUBSCRIBE_TEMPLATES.confirmed = 'T1'
      await requestSubscribe(['confirmed', 'reminder'])
      SUBSCRIBE_TEMPLATES.reminder = 'T2'
      await requestSubscribe(['confirmed', 'reminder'])
      expect(calls).toEqual([['T1'], ['T1', 'T2']])
    } finally {
      Object.assign(SUBSCRIBE_TEMPLATES, saved)
    }
  })
})
