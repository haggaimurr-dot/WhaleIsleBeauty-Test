/**
 * 真实后端实现（Go 服务）。演示阶段不使用。
 *
 * 部署在微信云托管，通过 wx.cloud.callContainer 调用：不用域名备案和 HTTPS 证书，
 * 网关会在每个请求上带上 X-WX-OPENID，后端据此识别客人，前端不用管 token。
 * 部署配置（环境 ID、服务名、传输方式）在 config.ts。换成自己的域名时把 TRANSPORT 改成 'https'，登录会自动走 code 换 token。
 * 云托管的 callContainer 需要基础库 2.23.0 以上，在小程序后台把最低基础库设到这个版本。
 */
import { API_PREFIX, BASE_URL, CLOUD_ENV, CLOUD_SERVICE, TRANSPORT } from './config'
import type { Api } from './contract'
import { ApiError, type Booking, type ErrorCode, type ID, type Me, type WxPayParams } from './types'

const TOKEN_KEY = 'jy_token'

type Method = 'GET' | 'POST' | 'PUT' | 'DELETE'

interface RawResponse { statusCode: number; data: unknown }

/** 只声明用到的部分，不引入额外类型包 */
interface WxCloud {
  init(opt?: { env?: string; traceUser?: boolean }): void
  callContainer(opt: {
    config: { env: string }
    path: string
    method: Method
    header: Record<string, string>
    data?: unknown
    success: (res: RawResponse) => void
    fail: (err: unknown) => void
  }): void
}
declare const wx: { cloud?: WxCloud }

let cloudReady = false

function transport(method: Method, path: string, data: unknown, header: Record<string, string>): Promise<RawResponse> {
  return new Promise((resolve, reject) => {
    if (TRANSPORT === 'cloud') {
      const cloud = wx.cloud
      if (!cloud) return reject(new Error('wx.cloud 不可用，基础库版本太低'))
      if (!cloudReady) { cloud.init({ env: CLOUD_ENV }); cloudReady = true }
      cloud.callContainer({
        config: { env: CLOUD_ENV },
        path: API_PREFIX + path,
        method,
        header: { 'X-WX-SERVICE': CLOUD_SERVICE, 'content-type': 'application/json', ...header },
        data,
        success: resolve,
        fail: reject,
      })
    } else {
      uni.request({
        url: BASE_URL + API_PREFIX + path,
        method,
        data: data as UniApp.RequestOptions['data'],
        header,
        success: res => resolve({ statusCode: res.statusCode, data: res.data }),
        fail: reject,
      })
    }
  })
}

interface RawResult<T> { ok: true; data: T }
interface RawError { ok: false; error: ApiError }

/** 发一次请求，不处理登录。有 token 才带 Authorization（云托管模式通常没有） */
async function send<T>(method: Method, path: string, data: unknown): Promise<RawResult<T> | RawError> {
  const token = getToken()
  let res: RawResponse
  try {
    res = await transport(method, path, data, token ? { Authorization: `Bearer ${token}` } : {})
  } catch {
    return { ok: false, error: new ApiError('NETWORK', '') }
  }
  if (res.statusCode >= 200 && res.statusCode < 300) return { ok: true, data: res.data as T }
  const body = (res.data || {}) as { code?: ErrorCode; message?: string }
  const fallback: ErrorCode = res.statusCode === 401 ? 'UNAUTHORIZED' : 'UNKNOWN'
  return { ok: false, error: new ApiError(body.code ?? fallback, body.message ?? '') }
}

// ---------- 登录 ----------

const getToken = (): string | undefined => uni.getStorageSync(TOKEN_KEY) || undefined

/** 同一时间只跑一次登录：启动时的 login() 和过期后的重新登录都共用它 */
let loggingIn: Promise<Me> | undefined
/** 每登录成功一次加 1。0 表示这次启动还没登录过 */
let loginGen = 0

function doLogin(): Promise<Me> {
  loggingIn ??= (async () => {
    try {
      // 云托管靠网关带的 openid 识别客人，不用 code；https 模式用 code 换 token
      const body = TRANSPORT === 'cloud' ? {} : { code: (await uni.login({ provider: 'weixin' })).code }
      const res = await send<{ token?: string; me: Me }>('POST', '/auth/wx-login', body)
      if (!res.ok) throw res.error
      if (res.data.token) uni.setStorageSync(TOKEN_KEY, res.data.token)
      else uni.removeStorageSync(TOKEN_KEY)
      loginGen++
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
 * - 这次启动还没登录过（或启动时的登录还没回来）就先等登录
 * - 返回 401 时重新登录，再重试一次原来的请求；还是 401 就把 UNAUTHORIZED 抛给页面
 */
async function request<T>(method: Method, path: string, data?: unknown): Promise<T> {
  if (loggingIn) await loggingIn.catch(() => {})
  if (!loginGen) await doLogin()

  const sentGen = loginGen
  const first = await send<T>(method, path, data)
  if (first.ok) return first.data
  if (first.error.code !== 'UNAUTHORIZED') throw first.error

  // 等待期间别的请求可能已经重新登录过了，那就直接重试，不用再登录一次
  if (loggingIn) await loggingIn
  else if (loginGen === sentGen) {
    uni.removeStorageSync(TOKEN_KEY)
    await doLogin()
  }
  const retry = await send<T>(method, path, data)
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
  getSkinProfile: () => request('GET', '/me/skin-profile'),
  updateSkinProfile: (req) => request('PUT', '/me/skin-profile', req),

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
  resumePayment: (id) => request('POST', `/bookings/${id}/pay`),
  cancelBooking: (id) => request('POST', `/bookings/${id}/cancel`),
  rescheduleBooking: (id, req) => request('POST', `/bookings/${id}/reschedule`, req),

  listScheduleDates: () => request('GET', '/owner/dates'),
  getDaySchedule: (date) => request('GET', '/owner/schedule' + qs({ date })),
  confirmBooking: (id) => request('POST', `/owner/bookings/${id}/confirm`),
  blockSlot: (artistId, date, time) => request('PUT', '/owner/blocks', { artistId, date, time }),
  unblockSlot: (artistId, date, time) => request('DELETE', '/owner/blocks', { artistId, date, time }),
  getOwnerNotify: () => request('GET', '/owner/notify'),
  addOwnerNotify: (count) => request('POST', '/owner/notify', { count }),

  updateShop: (shop) => request('PUT', '/owner/shop', shop),
  createArtist: (req) => request('POST', '/owner/artists', req),
  updateArtist: (id, req) => request('PUT', `/owner/artists/${id}`, req),
  deleteArtist: (id) => request('DELETE', `/owner/artists/${id}`),
  listOwnerServices: () => request('GET', '/owner/services'),
  createService: (req) => request('POST', '/owner/services', req),
  updateService: (id, req) => request('PUT', `/owner/services/${id}`, req),
  deleteService: (id) => request('DELETE', `/owner/services/${id}`),
  listOwnerWorks: () => request('GET', '/owner/works'),
  createWork: (req) => request('POST', '/owner/works', req),
  updateWork: (id, req) => request('PUT', `/owner/works/${id}`, req),
  deleteWork: (id) => request('DELETE', `/owner/works/${id}`),
}

/**
 * 联调用：还没有微信支付商户号时，后端（PAY_MODE=fake）返回假的支付参数，
 * pay.ts 用弹窗代替微信支付，再调 fakePaid 模拟支付回调。接真支付后删掉这两个
 */
export const isFakePayment = (p: WxPayParams) => p.package.startsWith('prepay_id=fake_')
export const fakePaid = (id: ID) => request<Booking>('POST', `/bookings/${id}/fake-paid`)
