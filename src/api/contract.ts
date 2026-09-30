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
  /**
   * POST /auth/wx-login  body: { code }  →  { token, me }，前端缓存 token
   * 其他接口 token 过期或无效时返回 401 UNAUTHORIZED，前端会重新登录并重试一次原请求
   */
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
  /**
   * GET /slots?artistId=&date=&serviceId=&excludeBookingId=
   * excludeBookingId：改期时传入正在改的预约，它自己占着的时段按可约返回。
   * 后端需校验该预约属于当前用户且未结束，否则忽略这个参数。
   */
  listSlots(artistId: ID, date: DateStr, serviceId: ID, excludeBookingId?: ID): Promise<SlotView[]>
  /**
   * POST /bookings
   * 后端需保证同一 (artistId, date, time) 不会被重复预约，冲突返回 409 SLOT_TAKEN
   */
  createBooking(req: CreateBookingReq): Promise<CreateBookingResp>
  /** GET /bookings/:id  支付后轮询状态用 */
  getBooking(id: ID): Promise<Booking>
  /**
   * GET /bookings/mine?scope=upcoming|past
   * upcoming：pending_payment、pending_confirm、confirmed，按开始时间升序
   * past：completed、cancelled，按开始时间降序。超时未付的 pending_payment 已被自动取消，出现在 past
   */
  listMyBookings(scope: 'upcoming' | 'past'): Promise<Booking[]>
  /**
   * POST /bookings/:id/pay  继续付定金：给 pending_payment 的预约重新生成支付参数
   * 已超过 payDeadline 或状态不是 pending_payment 时返回 INVALID_STATE
   */
  resumePayment(id: ID): Promise<CreateBookingResp>
  /** POST /bookings/:id/cancel  距离开始不足 24 小时返回 CANCEL_TOO_LATE（pending_payment 不受限制） */
  cancelBooking(id: ID): Promise<Booking>
  /** POST /bookings/:id/reschedule  同样受 24 小时规则限制，改期后回到 pending_confirm */
  rescheduleBooking(id: ID, req: RescheduleReq): Promise<Booking>

  // ---------- 排班（店主端，需要 owner 角色） ----------
  /** GET /owner/dates  排班可查看的日期（从今天起），和客人端的 /dates 不同，包含今天 */
  listScheduleDates(): Promise<DateStr[]>
  /** GET /owner/schedule?date= */
  getDaySchedule(date: DateStr): Promise<DaySchedule>
  /** POST /owner/bookings/:id/confirm  确认后给客人发订阅消息。开始时间已过返回 INVALID_STATE */
  confirmBooking(id: ID): Promise<Booking>
  /** PUT /owner/blocks  body: { artistId, date, time }  时段已过去或已有预约返回 INVALID_STATE */
  blockSlot(artistId: ID, date: DateStr, time: TimeStr): Promise<void>
  /** DELETE /owner/blocks  body: { artistId, date, time }  时段已过去返回 INVALID_STATE */
  unblockSlot(artistId: ID, date: DateStr, time: TimeStr): Promise<void>
}
