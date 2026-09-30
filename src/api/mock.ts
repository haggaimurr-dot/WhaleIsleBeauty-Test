/**
 * 内存 mock 实现。行为尽量贴近真实后端：有网络延迟、有状态流转、有冲突校验。
 * 页面不要直接 import 这个文件，统一从 '@/api' 获取。
 */
import type { Api } from './contract'
import {
  ApiError, type Artist, type Booking, type BookingStatus, type CreateBookingReq,
  type DateStr, type DaySchedule, type ID, type Me, type OwnerBookingBrief,
  type ScheduleCell, type Service, type Shop, type SlotView, type TimeStr, type Work,
} from './types'
import { upcomingDates, datesFromToday, toTimestamp, hoursUntil, todayStr, nowTimeStr } from '../utils/date'

// ---------- 种子数据 ----------

const TIMES: TimeStr[] = ['09:00', '10:30', '12:00', '13:30', '15:00', '16:30', '18:00', '19:30']

const artists: Artist[] = [
  { id: 'a1', name: '小鲸', title: '主理人', years: 8, avatar: 'placeholder:g1' },
  { id: 'a2', name: '安安', years: 6, specialty: '擅长新娘', avatar: 'placeholder:g4' },
  { id: 'a3', name: '七七', years: 4, specialty: '擅长日常', avatar: 'placeholder:g3' },
]

const services: Service[] = [
  {
    id: 's1', name: '韩式上镜妆（含发型）', summary: '适合拍照、证件照', category: 'camera',
    durationMin: 90, price: 29800, deposit: 5000,
    includes: ['妆前护肤和底妆', '眼妆、修容、唇妆', '简单盘发或卷发', '假睫毛可选，不另收费'],
    tags: ['拍照不假面', '氧气感底妆', '含简单盘发'],
    cover: 'placeholder:g2', images: ['placeholder:g2', 'placeholder:g6', 'placeholder:g4'], bookedCount: 128,
  },
  {
    id: 's2', name: '日常约会妆', summary: '可教你日常怎么画', category: 'date',
    durationMin: 60, price: 16800, deposit: 3000,
    includes: ['妆前护肤和底妆', '眼妆和唇妆', '化妆师讲解日常画法'],
    tags: ['近看也干净', '可学可带走'],
    cover: 'placeholder:g4', images: ['placeholder:g4', 'placeholder:g2'], bookedCount: 96,
  },
  {
    id: 's3', name: '新娘跟妆', summary: '需提前沟通，含试妆一次', category: 'bridal',
    durationMin: 180, price: 128000, priceFrom: true, deposit: 30000,
    includes: ['试妆一次', '婚礼当天早妆', '全天跟妆补妆', '造型更换两次'],
    tags: ['持妆一整天', '含试妆'],
    cover: 'placeholder:g3', images: ['placeholder:g3', 'placeholder:g1'], bookedCount: 41,
  },
  {
    id: 's4', name: '主持/年会妆', summary: '适合舞台灯光', category: 'host',
    durationMin: 90, price: 23800, deposit: 5000,
    includes: ['舞台底妆', '立体修容', '眼妆和唇妆', '简单发型'],
    tags: ['舞台灯光下', '上镜不反光'],
    cover: 'placeholder:g5', images: ['placeholder:g5'], bookedCount: 57,
  },
]

const works: Work[] = [
  { id: 'w1', title: '氧气上镜妆', category: 'camera', artistId: 'a1', serviceId: 's1', image: 'placeholder:g2', ratio: 1.25, durationText: '约 90 分钟' },
  { id: 'w2', title: '清透约会妆', category: 'date', artistId: 'a3', serviceId: 's2', image: 'placeholder:g4', ratio: 1.5, durationText: '约 60 分钟' },
  { id: 'w3', title: '中式新娘妆', category: 'bridal', artistId: 'a2', serviceId: 's3', image: 'placeholder:g3', ratio: 1.6, durationText: '需提前沟通' },
  { id: 'w4', title: '年会主持妆', category: 'host', artistId: 'a1', serviceId: 's4', image: 'placeholder:g5', ratio: 1.3, durationText: '约 90 分钟' },
  { id: 'w5', title: '毕业照妆', category: 'camera', artistId: 'a3', serviceId: 's1', image: 'placeholder:g6', ratio: 1.45, durationText: '约 75 分钟' },
  { id: 'w6', title: '韩式新娘妆', category: 'bridal', artistId: 'a2', serviceId: 's3', image: 'placeholder:g1', ratio: 1.25, durationText: '需提前沟通' },
  { id: 'w7', title: '面试淡妆', category: 'date', artistId: 'a3', serviceId: 's2', image: 'placeholder:g2', ratio: 1.4, durationText: '约 45 分钟' },
  { id: 'w8', title: '证件照妆', category: 'camera', artistId: 'a1', serviceId: 's1', image: 'placeholder:g4', ratio: 1.2, durationText: '约 60 分钟' },
]

/** 地址取自原型；电话和坐标是演示用的假数据 */
const shop: Shop = {
  name: '鲸屿美妆', address: '蓝山CBD 3329', phone: '020-0000-0000', openHours: '09:00–21:00',
  latitude: 23.1291, longitude: 113.2644,
}

/** 演示时同一个人既是客人也是店主，方便一台手机走完整流程。改成 'customer' 可隐藏店主入口 */
const me: Me = { id: 'u1', nickname: '阿柚', avatar: 'placeholder:g4', role: 'owner', visitCount: 0, skinProfile: '敏感肌 · 冷白皮' }

const OTHER_NAMES = ['林小姐', '陈小姐', '周小姐', '许小姐', '黄小姐', '吴小姐', '郑小姐', '何小姐']
const OTHER_SERVICES = ['上镜妆', '约会妆', '新娘试妆', '主持妆', '面试妆']
const ACTIVE: BookingStatus[] = ['pending_payment', 'pending_confirm', 'confirmed']
const PAY_WINDOW_MS = 15 * 60 * 1000

// ---------- 工具 ----------

const delay = (ms = 250 + Math.random() * 250) => new Promise(r => setTimeout(r, ms))
const clone = <T>(v: T): T => JSON.parse(JSON.stringify(v))
const slotKey = (artistId: ID, date: DateStr, time: TimeStr) => `${artistId}|${date}|${time}`

function hash(s: string) {
  let h = 0
  for (let i = 0; i < s.length; i++) h = (h * 31 + s.charCodeAt(i)) | 0
  return Math.abs(h)
}

/** 模拟“其他客人”已约走的时段：结果稳定，约 30% */
function takenByOthers(artistId: ID, date: DateStr, time: TimeStr) {
  return hash(slotKey(artistId, date, time)) % 10 < 3
}

function getOrThrow<T extends { id: ID }>(list: T[], id: ID): T {
  const item = list.find(x => x.id === id)
  if (!item) throw new ApiError('NOT_FOUND', '没有找到这条信息')
  return item
}

function artistName(id: ID) { return getOrThrow(artists, id).name }

// ---------- 可变状态 ----------

const bookings: Booking[] = (
  [
    ['s3', 'a2', '中式新娘试妆', '2026-08-16'],
    ['s2', 'a3', '日常约会妆', '2026-07-02'],
    ['s1', 'a1', '韩式上镜妆', '2026-05-20'],
  ] as const
).map(([serviceId, artistId, serviceName, date], i): Booking => ({
  id: `b${i + 1}`, serviceId, artistId, serviceName, artistName: artistName(artistId),
  date, time: '13:30', durationMin: 90, price: 29800, deposit: 5000,
  status: 'completed', createdAt: `${date}T08:00:00+08:00`, canCancel: false,
}))
const blocks = new Set<string>()
let seq = 100

/** 模拟后端的定时任务：超过付款截止时间的 pending_payment 自动取消，释放时段 */
function expireUnpaid() {
  const now = Date.now()
  for (const b of bookings) {
    if (b.status === 'pending_payment' && b.payDeadline && Date.parse(b.payDeadline) <= now) {
      b.status = 'cancelled'
      delete b.payDeadline
    }
  }
}

function getBookingOrThrow(id: ID) {
  expireUnpaid()
  return getOrThrow(bookings, id)
}

function payParams(b: Booking) {
  return {
    timeStamp: String(Math.floor(Date.now() / 1000)), nonceStr: 'mock',
    package: `prepay_id=mock_${b.id}`, signType: 'RSA' as const, paySign: 'mock',
  }
}

function findActive(artistId: ID, date: DateStr, time: TimeStr) {
  expireUnpaid()
  return bookings.find(b => b.artistId === artistId && b.date === date && b.time === time && ACTIVE.includes(b.status))
}

function isPast(date: DateStr, time: TimeStr) {
  return date < todayStr() || (date === todayStr() && time <= nowTimeStr())
}

function isAvailable(artistId: ID, date: DateStr, time: TimeStr, ignoreBookingId?: ID) {
  const active = findActive(artistId, date, time)
  return !isPast(date, time)
    && !takenByOthers(artistId, date, time)
    && !blocks.has(slotKey(artistId, date, time))
    && (!active || active.id === ignoreBookingId)
}

function out(b: Booking): Booking {
  const c = clone(b)
  c.canCancel = c.status === 'pending_payment' || (ACTIVE.includes(c.status) && hoursUntil(c.date, c.time) >= 24)
  return c
}

/** visitCount 按契约由预约记录算出：completed 的条数 */
function outMe(): Me {
  return { ...clone(me), visitCount: bookings.filter(b => b.status === 'completed').length }
}

function requireOwner() {
  if (me.role !== 'owner') throw new ApiError('FORBIDDEN', '只有店主可以进行这个操作')
}

/** mock 专用：模拟微信支付回调。真实环境由微信支付通知后端完成状态变更。 */
export async function __simulatePaid(id: ID) {
  await delay(600)
  const b = getBookingOrThrow(id)
  if (b.status === 'pending_payment') {
    b.status = 'pending_confirm'
    delete b.payDeadline
  }
}

// ---------- 实现 ----------

export const mockApi: Api = {
  async login() { await delay(); return outMe() },
  async getMe() { await delay(100); return outMe() },

  async listArtists() { await delay(); return clone(artists) },
  async listServices() { await delay(); return clone(services) },
  async getService(id) { await delay(); return clone(getOrThrow(services, id)) },
  async listWorks(category) {
    await delay()
    return clone(category ? works.filter(w => w.category === category) : works)
  },
  async listBookableDates() { await delay(100); return upcomingDates(7) },
  async getShop() { await delay(100); return clone(shop) },

  async listSlots(artistId, date, _serviceId, excludeBookingId): Promise<SlotView[]> {
    await delay()
    // 只认还在进行中的预约；mock 里只有一个用户，不用再校验归属
    const exclude = bookings.find(b => b.id === excludeBookingId && ACTIVE.includes(b.status))?.id
    return TIMES.map(time => ({ time, available: isAvailable(artistId, date, time, exclude) }))
  },

  async createBooking(req: CreateBookingReq) {
    await delay(500)
    const service = getOrThrow(services, req.serviceId)
    getOrThrow(artists, req.artistId)
    if (!isAvailable(req.artistId, req.date, req.time)) {
      throw new ApiError('SLOT_TAKEN', '这个时间刚被约走了，换一个时间吧')
    }
    const booking: Booking = {
      id: `b${++seq}`,
      serviceId: service.id, serviceName: service.name,
      artistId: req.artistId, artistName: artistName(req.artistId),
      date: req.date, time: req.time, durationMin: service.durationMin,
      price: service.price, deposit: service.deposit,
      status: 'pending_payment',
      occasion: req.occasion, skinType: req.skinType, note: req.note,
      createdAt: toTimestamp(new Date()), canCancel: true,
      payDeadline: toTimestamp(new Date(Date.now() + PAY_WINDOW_MS)),
    }
    bookings.push(booking)
    return { booking: out(booking), payment: payParams(booking) }
  },

  async getBooking(id) { await delay(150); return out(getBookingOrThrow(id)) },

  async resumePayment(id) {
    await delay(300)
    const b = getBookingOrThrow(id)
    if (b.status !== 'pending_payment') {
      throw new ApiError('INVALID_STATE', b.status === 'cancelled'
        ? '超过 15 分钟没付定金，这个时段已经放出去了，重新约一次吧'
        : '这个预约已经付过定金了')
    }
    return { booking: out(b), payment: payParams(b) }
  },

  async listMyBookings(scope) {
    await delay()
    const upcoming = scope === 'upcoming'
    expireUnpaid()
    const wanted: BookingStatus[] = upcoming ? ACTIVE : ['completed', 'cancelled']
    return bookings
      .filter(b => wanted.includes(b.status))
      .sort((a, b) => (a.date + a.time).localeCompare(b.date + b.time) * (upcoming ? 1 : -1))
      .map(out)
  },

  async cancelBooking(id) {
    await delay()
    const b = getBookingOrThrow(id)
    if (!ACTIVE.includes(b.status)) throw new ApiError('INVALID_STATE', '这个预约的状态已经变了')
    if (!out(b).canCancel) throw new ApiError('CANCEL_TOO_LATE', '距离开始不到 24 小时，需要取消请直接联系门店')
    b.status = 'cancelled'
    delete b.payDeadline
    return out(b)
  },

  async rescheduleBooking(id, req) {
    await delay(400)
    const b = getBookingOrThrow(id)
    if (!ACTIVE.includes(b.status)) throw new ApiError('INVALID_STATE', '这个预约的状态已经变了')
    if (!out(b).canCancel) throw new ApiError('CANCEL_TOO_LATE', '距离开始不到 24 小时，需要改期请直接联系门店')
    if (!isAvailable(req.artistId, req.date, req.time, b.id)) {
      throw new ApiError('SLOT_TAKEN', '这个时间刚被约走了，换一个时间吧')
    }
    Object.assign(b, {
      artistId: req.artistId, artistName: artistName(req.artistId),
      date: req.date, time: req.time, status: 'pending_confirm',
    })
    return out(b)
  },

  async listScheduleDates() {
    await delay(100)
    requireOwner()
    return datesFromToday(8)
  },

  async getDaySchedule(date): Promise<DaySchedule> {
    await delay()
    requireOwner()
    const cells: ScheduleCell[] = []
    const stats = { total: 0, pending: 0, free: 0, stale: 0 }
    for (const time of TIMES) {
      for (const a of artists) {
        const cell: ScheduleCell = { artistId: a.id, time, state: 'free' }
        const real = findActive(a.id, date, time)
        if (real && real.status !== 'pending_payment') {
          const brief: OwnerBookingBrief = {
            id: real.id, customerName: me.nickname, serviceName: real.serviceName, status: real.status,
            alert: real.skinType === 'sensitive' ? '敏感肌' : undefined, note: real.note,
            canConfirm: real.status === 'pending_confirm' && !isPast(real.date, real.time),
          }
          cell.state = real.status === 'pending_confirm' ? 'pending' : 'booked'
          cell.booking = brief
        } else if (takenByOthers(a.id, date, time)) {
          const h = hash(slotKey(a.id, date, time))
          const sensitive = h % 5 === 0
          cell.state = 'booked'
          cell.booking = {
            id: `other-${h}`, customerName: OTHER_NAMES[h % OTHER_NAMES.length],
            serviceName: OTHER_SERVICES[h % OTHER_SERVICES.length], status: 'confirmed',
            alert: sensitive ? '敏感肌' : undefined, note: sensitive ? '对酒精过敏' : undefined,
            canConfirm: false,
          }
        } else if (isPast(date, time)) {
          cell.state = 'past'
        } else if (blocks.has(slotKey(a.id, date, time))) {
          cell.state = 'blocked'
        }
        if (cell.state === 'booked' || cell.state === 'pending') stats.total++
        if (cell.state === 'pending') stats.pending++
        if (cell.state === 'pending' && !cell.booking?.canConfirm) stats.stale++
        if (cell.state === 'free') stats.free++
        cells.push(cell)
      }
    }
    return { date, times: [...TIMES], artists: artists.map(({ id, name }) => ({ id, name })), cells, stats }
  },

  async confirmBooking(id) {
    await delay()
    requireOwner()
    const b = getBookingOrThrow(id)
    if (b.status !== 'pending_confirm') throw new ApiError('INVALID_STATE', '这个预约已经处理过了')
    if (isPast(b.date, b.time)) throw new ApiError('INVALID_STATE', '预约时间已经过了，不能再确认')
    b.status = 'confirmed'
    return out(b)
  },

  async blockSlot(artistId, date, time) {
    await delay(200)
    requireOwner()
    if (isPast(date, time)) throw new ApiError('INVALID_STATE', '这个时段已经过去了')
    if (findActive(artistId, date, time) || takenByOthers(artistId, date, time)) {
      throw new ApiError('INVALID_STATE', '这个时段已经有预约了')
    }
    blocks.add(slotKey(artistId, date, time))
  },

  async unblockSlot(artistId, date, time) {
    await delay(200)
    requireOwner()
    if (isPast(date, time)) throw new ApiError('INVALID_STATE', '这个时段已经过去了')
    blocks.delete(slotKey(artistId, date, time))
  },
}
