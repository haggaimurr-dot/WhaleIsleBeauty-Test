/**
 * 真实后端实现（Go 服务）。演示阶段不使用。
 * 如果部署在微信云托管，可以把 uni.request 换成 wx.cloud.callContainer，免去域名备案和 HTTPS 配置。
 */
import type { Api } from './contract'
import { ApiError, type ErrorCode, type Me } from './types'

const BASE_URL = 'https://api.example.com/v1' // TODO: 换成真实地址
const TOKEN_KEY = 'jy_token'

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

function request<T>(method: Method, path: string, data?: unknown): Promise<T> {
  return new Promise((resolve, reject) => {
    uni.request({
      url: BASE_URL + path,
      method,
      data: data as UniApp.RequestOptions['data'],
      header: { Authorization: `Bearer ${uni.getStorageSync(TOKEN_KEY) || ''}` },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) return resolve(res.data as T)
        const body = (res.data || {}) as { code?: ErrorCode; message?: string }
        const fallback: ErrorCode = res.statusCode === 401 ? 'UNAUTHORIZED' : 'UNKNOWN'
        reject(new ApiError(body.code ?? fallback, body.message ?? ''))
      },
      fail: () => reject(new ApiError('NETWORK', '')),
    })
  })
}

const qs = (o: Record<string, string | undefined>) => {
  const s = Object.entries(o)
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => `${k}=${encodeURIComponent(v as string)}`)
    .join('&')
  return s ? `?${s}` : ''
}

export const httpApi: Api = {
  async login() {
    const { code } = await uni.login({ provider: 'weixin' })
    const res = await request<{ token: string; me: Me }>('POST', '/auth/wx-login', { code })
    uni.setStorageSync(TOKEN_KEY, res.token)
    return res.me
  },
  getMe: () => request('GET', '/me'),

  listArtists: () => request('GET', '/artists'),
  listServices: () => request('GET', '/services'),
  getService: (id) => request('GET', `/services/${id}`),
  listWorks: (category) => request('GET', '/works' + qs({ category })),
  listBookableDates: () => request('GET', '/dates'),
  getShop: () => request('GET', '/shop'),

  listSlots: (artistId, date, serviceId) => request('GET', '/slots' + qs({ artistId, date, serviceId })),
  createBooking: (req) => request('POST', '/bookings', req),
  getBooking: (id) => request('GET', `/bookings/${id}`),
  listMyBookings: (scope) => request('GET', '/bookings/mine' + qs({ scope })),
  cancelBooking: (id) => request('POST', `/bookings/${id}/cancel`),
  rescheduleBooking: (id, req) => request('POST', `/bookings/${id}/reschedule`, req),

  getDaySchedule: (date) => request('GET', '/owner/schedule' + qs({ date })),
  confirmBooking: (id) => request('POST', `/owner/bookings/${id}/confirm`),
  blockSlot: (artistId, date, time) => request('PUT', '/owner/blocks', { artistId, date, time }),
  unblockSlot: (artistId, date, time) => request('DELETE', '/owner/blocks', { artistId, date, time }),
}
