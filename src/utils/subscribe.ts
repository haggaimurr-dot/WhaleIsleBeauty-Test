/**
 * 订阅消息（一次性订阅）。客人点一次、同意一次，后端才能给他发一条对应的消息。
 *
 * - 预约确认：店主在工作台点“确认”后发（confirmBooking）
 * - 到店提醒：预约开始前一天 18:00 以后由后端定时任务发（只发给已确认的预约）
 *
 * 后端实现在 server/notify.go，模板 ID 和下面的字段两边要一致。
 *
 * 必须在点击事件里同步发起，前面不能有 await，否则微信会报“只能由用户点击触发”。
 * 客人拒绝、关掉弹窗或调用失败都不影响预约本身，所以这里永远 resolve。
 */

/**
 * 小程序后台「订阅消息 → 我的模板」里的模板 ID。留空的模板会跳过，全部为空时不弹窗。
 *
 * 后端发送时 data 的 key 必须和下面一致（以「我的模板」详情为准，和公共模板库里的编号不同）。
 * thing 类最多 20 个字，超出要截断；date / time 用“2026年10月01日 14:00”这种格式。
 *
 * confirmed — 预约成功通知（公共模板 28772），店主确认后发
 *   thing8   预约项目   服务名，如“新娘跟妆”
 *   date17   预约时间   “2026年10月01日 14:00”
 *   thing14  预约门店   店名 + 化妆师，如“鲸屿化妆室 · 小鱼”
 *   thing19  备注       如“素颜过来就好”
 *
 * reminder — 日程安排提醒（公共模板 29816），开始前一天发
 *   thing1   日程名称   服务名，如“新娘跟妆”
 *   time3    开始时间   “2026年10月01日 14:00”
 *   thing6   地点       门店地址
 *   thing5   日程描述   如“明天见，素颜过来就好”
 */
export const SUBSCRIBE_TEMPLATES: Record<'confirmed' | 'reminder', string> = {
  confirmed: 'R6MUu_p5k6R-LPbVgm60btOsqWDVdb3Rbpykw8hH08Q',
  reminder: 'TpVpZ-EaOftX3_nW3RFYtx1BiBwpZbJRtJEfifCeM2Y',
}

export type SubscribeKind = keyof typeof SUBSCRIBE_TEMPLATES

export function requestSubscribe(kinds: SubscribeKind[]): Promise<void> {
  const tmplIds = kinds.map(k => SUBSCRIBE_TEMPLATES[k]).filter(Boolean)
  if (!tmplIds.length) return Promise.resolve()
  return new Promise(resolve => {
    uni.requestSubscribeMessage({ tmplIds, complete: () => resolve() })
  })
}
