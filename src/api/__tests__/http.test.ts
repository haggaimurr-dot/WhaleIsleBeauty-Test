/** 真实后端的请求层：云托管和自有域名两种传输方式，登录、401 重试、错误映射 */
import { afterEach, describe, expect, it, vi } from 'vitest'

type Header = Record<string, string>
interface Req { method: string; path: string; header: Header; data: unknown }

/** 假后端：按 token（https）或网关（云托管）认人 */
function fakeServer(issueToken: boolean) {
  const st = {
    log: [] as Req[],
    valid: '' as string,
    seq: 0,
    forced: [] as [string, number][],
    handle(r: Req): { statusCode: number; data: unknown } {
      st.log.push(r)
      if (r.path === '/v1/auth/wx-login') {
        if (!issueToken) return { statusCode: 200, data: { me: { id: 'u1' } } }
        st.valid = `t${++st.seq}`
        return { statusCode: 200, data: { token: st.valid, me: { id: 'u1' } } }
      }
      const f = st.forced.shift()
      if (f) return { statusCode: f[1], data: { code: f[0], message: '' } }
      if (issueToken && r.header.Authorization !== `Bearer ${st.valid}`) return { statusCode: 401, data: {} }
      return { statusCode: 200, data: { path: r.path } }
    },
    logins: () => st.log.filter(r => r.path === '/v1/auth/wx-login').length,
  }
  return st
}

function stubUni(extra: Record<string, unknown> = {}) {
  const store: Record<string, string> = {}
  const uni = {
    getStorageSync: (k: string) => store[k] ?? '',
    setStorageSync: (k: string, v: string) => { store[k] = v },
    removeStorageSync: (k: string) => { delete store[k] },
    login: vi.fn(async () => ({ code: 'c' })),
    ...extra,
  }
  vi.stubGlobal('uni', uni)
  return { uni, store }
}

const later = (fn: () => void) => setTimeout(fn, 1)
const code = (e: unknown) => (e as { code?: string })?.code

/** 按传输方式加载一份全新的 http.ts */
async function loadHttp(transport: 'cloud' | 'https') {
  vi.resetModules()
  vi.doMock('@/api/config', () => ({
    TRANSPORT: transport, CLOUD_ENV: 'env-test', CLOUD_SERVICE: 'jingyu', BASE_URL: 'https://api.test', API_PREFIX: '/v1',
  }))
  return (await import('@/api/http')).httpApi
}

afterEach(() => {
  vi.unstubAllGlobals()
  vi.doUnmock('@/api/config')
})

describe('云托管（callContainer）', () => {
  function setup(scene = 1001) {
    const srv = fakeServer(false)
    const { uni } = stubUni({ getLaunchOptionsSync: () => ({ scene }) })
    const cloud = {
      init: vi.fn(),
      callContainer: vi.fn((o: { path: string; method: string; header: Header; data: unknown; config: { env: string }; success: (r: unknown) => void }) =>
        later(() => o.success(srv.handle({ method: o.method, path: o.path, header: o.header, data: o.data })))),
    }
    vi.stubGlobal('wx', { cloud })
    return { srv, uni, cloud }
  }

  it('不调用 uni.login、不带 token、带服务名；并发请求只登录一次', async () => {
    const { srv, uni, cloud } = setup()
    const api = await loadHttp('cloud')
    const [me, r] = await Promise.all([api.login(), api.getMe(), api.listArtists()])
    expect(me).toEqual({ id: 'u1' })
    expect(r).toEqual({ path: '/v1/me' })
    expect(uni.login).not.toHaveBeenCalled()
    expect(srv.logins()).toBe(1)
    expect(srv.log.every(x => !x.header.Authorization && x.header['X-WX-SERVICE'] === 'jingyu')).toBe(true)
    expect(cloud.init).toHaveBeenCalledTimes(1)
    expect(cloud.callContainer.mock.calls[0][0].config.env).toBe('env-test')
  })

  it('没有 token 也不会每次请求都去登录；401 时重新登录再重试一次', async () => {
    const { srv } = setup()
    const api = await loadHttp('cloud')
    await api.getMe(); await api.getShop(); await api.listServices()
    expect(srv.logins()).toBe(1)
    srv.forced = [['UNAUTHORIZED', 401]]
    expect(await api.getMe()).toEqual({ path: '/v1/me' })
    expect(srv.logins()).toBe(2)
  })

  it('朋友圈单页模式（1154）不登录，401 直接抛 UNAUTHORIZED，不重试', async () => {
    const { srv, cloud } = setup(1154)
    const api = await loadHttp('cloud')
    expect(await api.getService('s1')).toEqual({ path: '/v1/services/s1' })
    expect(await api.listWorks('daily')).toEqual({ path: '/v1/works?category=daily' })
    srv.forced = [['UNAUTHORIZED', 401]]
    await expect(api.listMyBookings('upcoming')).rejects.toSatisfy(e => code(e) === 'UNAUTHORIZED')
    expect(srv.logins()).toBe(0)
    expect(cloud.callContainer).toHaveBeenCalledTimes(3)
  })

  it('调用失败报 NETWORK；没有 wx.cloud 也报 NETWORK', async () => {
    const { cloud } = setup()
    const api = await loadHttp('cloud')
    cloud.callContainer.mockImplementation((o: { fail: (e: unknown) => void }) => later(() => o.fail({ errMsg: 'x' })))
    await expect(api.getMe()).rejects.toSatisfy(e => code(e) === 'NETWORK')
    vi.stubGlobal('wx', {})
    const api2 = await loadHttp('cloud')
    await expect(api2.getMe()).rejects.toSatisfy(e => code(e) === 'NETWORK')
  })
})

describe('自有域名（uni.request）', () => {
  function setup() {
    const srv = fakeServer(true)
    const request = vi.fn((o: { url: string; method: string; header: Header; data: unknown; success: (r: unknown) => void }) =>
      later(() => o.success(srv.handle({ method: o.method, path: o.url.replace('https://api.test', ''), header: o.header, data: o.data }))))
    const { uni, store } = stubUni({ request })
    vi.stubGlobal('wx', {})
    return { srv, uni, store, request }
  }

  it('code 换 token 并缓存；地址是 BASE_URL + /v1', async () => {
    const { uni, store, request } = setup()
    const api = await loadHttp('https')
    await Promise.all([api.login(), api.getMe()])
    expect(uni.login).toHaveBeenCalledTimes(1)
    expect(store.jy_token).toBe('t1')
    expect(request.mock.calls.every(c => c[0].url.startsWith('https://api.test/v1/'))).toBe(true)
  })

  it('token 失效：三个并发请求只重新登录一次', async () => {
    const { srv, uni, store } = setup()
    const api = await loadHttp('https')
    await api.getMe()
    srv.valid = 'rotated'
    await Promise.all([api.getMe(), api.listArtists(), api.getShop()])
    expect(uni.login).toHaveBeenCalledTimes(2)
    expect(store.jy_token).toBe('t2')
  })

  it('重试后还是 401 报 UNAUTHORIZED；其他错误不重试', async () => {
    const { srv } = setup()
    const api = await loadHttp('https')
    await api.getMe()
    srv.forced = [['UNAUTHORIZED', 401], ['UNAUTHORIZED', 401]]
    await expect(api.getMe()).rejects.toSatisfy(e => code(e) === 'UNAUTHORIZED')
    srv.forced = [['SLOT_TAKEN', 409]]
    const n = srv.log.length
    await expect(api.createBooking({ serviceId: 's1', artistId: 'a1', date: '2026-10-02', time: '09:00' }))
      .rejects.toSatisfy(e => code(e) === 'SLOT_TAKEN')
    expect(srv.log.length - n).toBe(1)
  })
})
