import type { ID, StyleCategory } from '../api/types'

/**
 * tabBar 页面之间跳转。uni.switchTab 不能带 query，
 * 所以跳转前把参数放在这里，目标页在 onShow 里用 takeTabParams 取一次。
 */
export type TabPage = 'home' | 'works' | 'booking' | 'me'

export interface TabParams {
  home: undefined
  works: { category?: StyleCategory }
  /** 预约同款、预约这个妆、再约一次时带过去，预约页据此预选；again 表示照着客人上次的预约选 */
  booking: { serviceId?: ID; artistId?: ID; again?: boolean } | undefined
  me: undefined
}

const TAB_URL: Record<TabPage, string> = {
  home: '/pages/home/index',
  works: '/pages/works/index',
  booking: '/pages/booking/index',
  me: '/pages/me/index',
}

const pending: { [K in TabPage]?: TabParams[K] } = {}

export function switchTab<K extends TabPage>(page: K, params?: TabParams[K]) {
  pending[page] = params
  uni.switchTab({ url: TAB_URL[page] })
}

/** 取出并清掉跳转参数，没有则返回 undefined */
export function takeTabParams<K extends TabPage>(page: K): TabParams[K] | undefined {
  const params = pending[page]
  delete pending[page]
  return params
}
