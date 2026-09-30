/**
 * 订阅消息（一次性订阅）。客人点一次、同意一次，后端才能给他发一条对应的消息。
 *
 * - 预约确认：店主在工作台点“确认”后发（confirmBooking）
 * - 到店提醒：预约开始前由后端定时发
 *
 * 必须在点击事件里同步发起，前面不能有 await，否则微信会报“只能由用户点击触发”。
 * 客人拒绝、关掉弹窗或调用失败都不影响预约本身，所以这里永远 resolve。
 */

/** 在小程序后台「订阅消息」里申请后填入。留空的模板会跳过，全部为空时不弹窗（演示版就是这样） */
export const SUBSCRIBE_TEMPLATES: Record<'confirmed' | 'reminder', string> = {
  confirmed: '', // TODO: 预约确认
  reminder: '', // TODO: 到店提醒
}

export type SubscribeKind = keyof typeof SUBSCRIBE_TEMPLATES

export function requestSubscribe(kinds: SubscribeKind[]): Promise<void> {
  const tmplIds = kinds.map(k => SUBSCRIBE_TEMPLATES[k]).filter(Boolean)
  if (!tmplIds.length) return Promise.resolve()
  return new Promise(resolve => {
    uni.requestSubscribeMessage({ tmplIds, complete: () => resolve() })
  })
}
