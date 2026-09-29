import { api, USE_MOCK } from './client'
import { __simulatePaid } from './mock'
import { ApiError, type Booking, type CreateBookingResp } from './types'

/**
 * 定金支付流程：拉起微信支付 → 轮询预约状态，直到后端收到支付回调。
 * 用户中途取消支付时抛出 ApiError('INVALID_STATE')，页面可以直接显示 message。
 */
export async function payDeposit({ booking, payment }: CreateBookingResp): Promise<Booking> {
  if (USE_MOCK) {
    await __simulatePaid(booking.id)
  } else {
    try {
      await uni.requestPayment({ provider: 'wxpay', orderInfo: '', ...payment })
    } catch {
      throw new ApiError('INVALID_STATE', '还没付定金，这个时段会为你保留 15 分钟')
    }
  }
  // 前端支付成功不代表后端已收到回调，轮询几次
  for (let i = 0; i < 5; i++) {
    const latest = await api.getBooking(booking.id)
    if (latest.status !== 'pending_payment') return latest
    await new Promise(r => setTimeout(r, 800))
  }
  return api.getBooking(booking.id)
}
