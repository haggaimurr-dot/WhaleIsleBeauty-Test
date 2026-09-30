import { api, USE_MOCK } from './client'
import { __simulatePaid } from './mock'
import { fakePaid, isFakePayment } from './http'
import { ApiError, type Booking, type CreateBookingResp } from './types'
import { formatPrice } from '../utils/money'

const NOT_PAID = '还没付定金，时段为你保留 15 分钟，可以在「我的」里继续付'

/** 演示和联调用：用弹窗代替微信支付，选“先不付”可以走到待付定金的流程 */
function mockPayDialog(booking: Booking): Promise<boolean> {
  return new Promise(resolve => {
    uni.showModal({
      title: '演示支付',
      content: `定金 ${formatPrice(booking.deposit)}（演示版不会真的扣款）`,
      confirmText: '支付',
      cancelText: '先不付',
      confirmColor: '#6E5446',
      success: r => resolve(r.confirm),
      fail: () => resolve(false),
    })
  })
}

/**
 * 定金支付流程：拉起微信支付 → 轮询预约状态，直到后端收到支付回调。
 * 下单后第一次付、在「我的」里继续付都走这里（后者先调 api.resumePayment 拿参数）。
 * 用户中途取消支付时抛出 ApiError('INVALID_STATE')，页面可以直接显示 message。
 */
export async function payDeposit({ booking, payment }: CreateBookingResp): Promise<Booking> {
  if (USE_MOCK) {
    if (!(await mockPayDialog(booking))) throw new ApiError('INVALID_STATE', NOT_PAID)
    await __simulatePaid(booking.id)
  } else if (isFakePayment(payment)) {
    if (!(await mockPayDialog(booking))) throw new ApiError('INVALID_STATE', NOT_PAID)
    await fakePaid(booking.id)
  } else {
    try {
      await uni.requestPayment({ provider: 'wxpay', orderInfo: '', ...payment })
    } catch {
      throw new ApiError('INVALID_STATE', NOT_PAID)
    }
  }
  // 前端支付成功不代表后端已收到回调，轮询几次
  for (let i = 0; i < 5; i++) {
    const latest = await api.getBooking(booking.id)
    if (latest.status === 'cancelled') {
      throw new ApiError('INVALID_STATE', '超过 15 分钟没付定金，这个时段已经放出去了，重新约一次吧')
    }
    if (latest.status !== 'pending_payment') return latest
    await new Promise(r => setTimeout(r, 800))
  }
  return api.getBooking(booking.id)
}
