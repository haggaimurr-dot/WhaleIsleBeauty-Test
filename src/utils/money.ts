import type { Cents } from '../api/types'

/** 29800 → '¥298'，12850 → '¥128.5'，priceFrom 时加“起” */
export function formatPrice(cents: Cents, opts: { from?: boolean } = {}): string {
  const yuan = cents / 100
  const text = Number.isInteger(yuan) ? String(yuan) : yuan.toFixed(2).replace(/0$/, '')
  return `¥${text}${opts.from ? ' 起' : ''}`
}

/** 店主在输入框里填的元 → 分。'298' → 29800，'128.5' → 12850；最多两位小数，认不出的返回 NaN */
export function parseYuan(text: string): Cents {
  const m = /^(\d+)(?:\.(\d{1,2}))?$/.exec(text.trim())
  if (!m) return NaN
  // 按字符串拼，避免 19.99 * 100 这类浮点误差
  return Number(m[1]) * 100 + Number((m[2] ?? '').padEnd(2, '0'))
}

/** 分 → 输入框里的元，和 formatPrice 的写法一致但不带 ¥：29800 → '298' */
export function centsToYuanInput(cents: Cents): string {
  return formatPrice(cents).slice(1)
}
