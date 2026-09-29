import type { Cents } from '../api/types'

/** 29800 → '¥298'，12850 → '¥128.5'，priceFrom 时加“起” */
export function formatPrice(cents: Cents, opts: { from?: boolean } = {}): string {
  const yuan = cents / 100
  const text = Number.isInteger(yuan) ? String(yuan) : yuan.toFixed(2).replace(/0$/, '')
  return `¥${text}${opts.from ? ' 起' : ''}`
}
