import type {
  Artist, Booking, CreateBookingReq, CreateBookingResp, DateStr, DaySchedule,
  ID, Me, RescheduleReq, Service, Shop, SlotView, StyleCategory, TimeStr, Work,
} from './types'

/**
 * Api 契约。mock.ts 和 http.ts 都实现这个接口。
 * 注释里是对应的 REST 路径，Go 后端按这个实现。
 *
 * 约定：
 * - 成功：HTTP 2xx，body 直接是返回数据（不包 {code, data} 信封）
 * - 失败：HTTP 4xx/5xx，body 为 { "code": ErrorCode, "message": string }
 * - 鉴权：Header `Authorization: Bearer <token>`，token 由 /auth/wx-login 换取
 */
export interface Api {
  // ---------- 登录 ----------
  /** POST /auth/wx-login  body: { code }  →  { token, me }，前端缓存 token */
  login(): Promise<Me>
  /** GET /me */
  getMe(): Promise<Me>

  // ---------- 基础资料 ----------
  /** GET /artists */
  listArtists(): Promise<Artist[]>
  /** GET /services */
  listServices(): Promise<Service[]>
  /** GET /services/:id */
  getService(id: ID): Promise<Service>
  /** GET /works?category= */
  listWorks(category?: StyleCategory): Promise<Work[]>
  /** GET /dates  可预约的日期（从明天起），后端可据此处理节假日或闭店 */
  listBookableDates(): Promise<DateStr[]>
  /** GET /shop  门店信息 */
  getShop(): Promise<Shop>

  // ---------- 预约（客人端） ----------
  /** GET /slots?artistId=&date=&serviceId= */
  listSlots(artistId: ID, date: DateStr, serviceId: ID): Promise<SlotView[]>
  /**
   * POST /bookings
   * 后端需保证同一 (artistId, date, time) 不会被重复预约，冲突返回 409 SLOT_TAKEN
   */
  createBooking(req: CreateBookingReq): Promise<CreateBookingResp>
  /** GET /bookings/:id  支付后轮询状态用 */
  getBooking(id: ID): Promise<Booking>
  /** GET /bookings/mine?scope=upcoming|past */
  listMyBookings(scope: 'upcoming' | 'past'): Promise<Booking[]>
  /** POST /bookings/:id/cancel  距离开始不足 24 小时返回 CANCEL_TOO_LATE */
  cancelBooking(id: ID): Promise<Booking>
  /** POST /bookings/:id/reschedule  同样受 24 小时规则限制，改期后回到 pending_confirm */
  rescheduleBooking(id: ID, req: RescheduleReq): Promise<Booking>

  // ---------- 排班（店主端，需要 owner 角色） ----------
  /** GET /owner/schedule?date= */
  getDaySchedule(date: DateStr): Promise<DaySchedule>
  /** POST /owner/bookings/:id/confirm  确认后给客人发订阅消息 */
  confirmBooking(id: ID): Promise<Booking>
  /** PUT /owner/blocks  body: { artistId, date, time } */
  blockSlot(artistId: ID, date: DateStr, time: TimeStr): Promise<void>
  /** DELETE /owner/blocks  body: { artistId, date, time } */
  unblockSlot(artistId: ID, date: DateStr, time: TimeStr): Promise<void>
}
