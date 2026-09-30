/**
 * 真实后端实现（Go 服务）。演示阶段不使用。
 * 如果部署在微信云托管，可以把 uni.request 换成 wx.cloud.callContainer，免去域名备案和 HTTPS 配置。
 */
import type { Api } from './contract'
import { ApiError, type ErrorCode, type Me } from './types'

const BASE_URL = 'https://api.example.com/v1' // TODO: 换成真实地址
const TOKEN_KEY = 'jy_token'

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

interface RawResult<T> { ok: true; data: T }
interface RawError { ok: false; error: ApiError }

/** 发一次请求，不处理登录。token 为 undefined 时不带 Authorization */
function send<T>(method: Method, path: string, data: unknown, token?: string): Promise<RawResult<T> | RawError> {
  return new Promise(resolve => {
    uni.request({
      url: BASE_URL + path,
      method,
      data: data as UniApp.RequestOptions['data'],
      header: token ? { Authorization: `Bearer ${token}` } : {},
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) return resolve({ ok: true, data: res.data as T })
        const body = (res.data || {}) as { code?: ErrorCode; message?: string }
        const fallback: ErrorCode = res.statusCode === 401 ? 'UNAUTHORIZED' : 'UNKNOWN'
        resolve({ ok: false, error: new ApiError(body.code ?? fallback, body.message ?? '') })
      },
      fail: () => resolve({ ok: false, error: new ApiError('NETWORK', '') }),
    })
  })
}

// ---------- 登录 ----------

const getToken = (): string | undefined => uni.getStorageSync(TOKEN_KEY) || undefined

/** 同一时间只跑一次登录：启动时的 login() 和过期后的重新登录都共用它 */
let loggingIn: Promise<Me> | undefined

function doLogin(): Promise<Me> {
  loggingIn ??= (async () => {
    try {
      const { code } = await uni.login({ provider: 'weixin' })
      const res = await send<{ token: string; me: Me }>('POST', '/auth/wx-login', { code })
      if (!res.ok) throw res.error
      uni.setStorageSync(TOKEN_KEY, res.data.token)
      return res.data.me
    } catch (e) {
      // uni.login 自己失败（没网、微信没响应）也归到 NETWORK
      throw e instanceof ApiError ? e : new ApiError('NETWORK', '')
    } finally {
      loggingIn = undefined
    }
  })()
  return loggingIn
}

/**
 * 需要登录的请求：
 * - 还没有 token（或启动时的登录还没回来）就先等登录
 * - 返回 401 时重新登录，再重试一次原来的请求；还是 401 就把 UNAUTHORIZED 抛给页面
 */
async function request<T>(method: Method, path: string, data?: unknown): Promise<T> {
  if (loggingIn) await loggingIn.catch(() => {})
  if (!getToken()) await doLogin()

  const sentWith = getToken()
  const first = await send<T>(method, path, data, sentWith)
  if (first.ok) return first.data
  if (first.error.code !== 'UNAUTHORIZED') throw first.error

  // 等待期间别的请求可能已经换过 token 了，那就直接用新的重试，不用再登录一次
  if (loggingIn) await loggingIn
  else if (getToken() === sentWith) {
    uni.removeStorageSync(TOKEN_KEY)
    await doLogin()
  }
  const retry = await send<T>(method, path, data, getToken())
  if (retry.ok) return retry.data
  throw retry.error
}

const qs = (o: Record<string, string | undefined>) => {
  const s = Object.entries(o)
    .filter(([, v]) => v !== undefined)
    .map(([k, v]) => `${k}=${encodeURIComponent(v as string)}`)
    .join('&')
  return s ? `?${s}` : ''
}

export const httpApi: Api = {
  login: doLogin,
  getMe: () => request('GET', '/me'),

  listArtists: () => request('GET', '/artists'),
  listServices: () => request('GET', '/services'),
  getService: (id) => request('GET', `/services/${id}`),
  listWorks: (category) => request('GET', '/works' + qs({ category })),
  listBookableDates: () => request('GET', '/dates'),
  getShop: () => request('GET', '/shop'),

  listSlots: (artistId, date, serviceId, excludeBookingId) =>
    request('GET', '/slots' + qs({ artistId, date, serviceId, excludeBookingId })),
  createBooking: (req) => request('POST', '/bookings', req),
  getBooking: (id) => request('GET', `/bookings/${id}`),
  listMyBookings: (scope) => request('GET', '/bookings/mine' + qs({ scope })),
  cancelBooking: (id) => request('POST', `/bookings/${id}/cancel`),
  rescheduleBooking: (id, req) => request('POST', `/bookings/${id}/reschedule`, req),

  listScheduleDates: () => request('GET', '/owner/dates'),
  getDaySchedule: (date) => request('GET', '/owner/schedule' + qs({ date })),
  confirmBooking: (id) => request('POST', `/owner/bookings/${id}/confirm`),
  blockSlot: (artistId, date, time) => request('PUT', '/owner/blocks', { artistId, date, time }),
  unblockSlot: (artistId, date, time) => request('DELETE', '/owner/blocks', { artistId, date, time }),
}
