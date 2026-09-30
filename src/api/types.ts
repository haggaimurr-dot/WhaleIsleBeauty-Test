/**
 * 数据类型定义。这是前端和 Go 后端之间的契约：
 * 字段名使用 camelCase，Go 侧用 `json:"fieldName"` tag 对齐。
 * 已有字段不要改名或改含义，只能新增。
 */

export type ID = string
/** 'YYYY-MM-DD'，按门店所在时区（Asia/Shanghai） */
export type DateStr = string
/** 'HH:mm' */
export type TimeStr = string
/** 金额，单位：分 */
export type Cents = number
/** ISO 8601 时间戳 */
export type Timestamp = string

// ---------- 基础资料 ----------

export type StyleCategory = 'camera' | 'date' | 'bridal' | 'host'

export const CATEGORY_LABEL: Record<StyleCategory, string> = {
  camera: '上镜',
  date: '约会',
  bridal: '新娘',
  host: '主持',
}

export interface Artist {
  id: ID
  name: string
  /** 例如“主理人” */
  title?: string
  years: number
  /** 例如“擅长新娘” */
  specialty?: string
  avatar: string
}

export interface Service {
  id: ID
  name: string
  summary: string
  category: StyleCategory
  durationMin: number
  price: Cents
  /** true 时显示为“¥1280 起” */
  priceFrom?: boolean
  deposit: Cents
  includes: string[]
  tags: string[]
  cover: string
  images: string[]
  /** 已选择人数，用于详情页展示 */
  bookedCount: number
}

export interface Work {
  id: ID
  title: string
  category: StyleCategory
  artistId: ID
  serviceId?: ID
  image: string
  /** 宽高比（高/宽），用于瀑布流占位 */
  ratio: number
  durationText: string
}

/** 门店信息，用于成功页地址、联系门店、到店导航 */
export interface Shop {
  name: string
  address: string
  phone: string
  /** 例如 '09:00–21:00' */
  openHours: string
  /** GCJ-02 坐标，uni.openLocation 使用 */
  latitude: number
  longitude: number
}

// ---------- 用户 ----------

export type Role = 'customer' | 'owner'

export interface Me {
  id: ID
  nickname: string
  avatar: string
  role: Role
  /**
   * 已到店完成的次数：该用户状态为 completed 的预约数。
   * 不含已取消、待付定金和还没到的预约。前端显示“第 visitCount + 1 次来”。
   */
  visitCount: number
  /** 客人自己填写的档案，二期功能 */
  skinProfile?: string
}

// ---------- 预约（客人端） ----------

/** 客人端只能看到某个时段是否可约，看不到是谁约的 */
export interface SlotView {
  time: TimeStr
  available: boolean
}

export type Occasion = 'photo' | 'date' | 'interview' | 'event' | 'other'
export type SkinType = 'dry' | 'oily' | 'combination' | 'sensitive' | 'unsure'

export const OCCASION_LABEL: Record<Occasion, string> = {
  photo: '拍照', date: '约会', interview: '面试', event: '活动/主持', other: '其他',
}
export const SKIN_LABEL: Record<SkinType, string> = {
  dry: '干皮', oily: '油皮', combination: '混合', sensitive: '敏感', unsure: '不确定',
}

/**
 * 预约状态流转：
 * pending_payment --付定金--> pending_confirm --店主确认--> confirmed --到店完成--> completed
 * 除 completed 外都可以 --> cancelled
 * pending_payment 超过 15 分钟未支付，由后端自动取消并释放时段
 */
export type BookingStatus =
  | 'pending_payment'
  | 'pending_confirm'
  | 'confirmed'
  | 'completed'
  | 'cancelled'

export const STATUS_LABEL: Record<BookingStatus, string> = {
  pending_payment: '待付定金',
  pending_confirm: '等待店里确认',
  confirmed: '店里已确认',
  completed: '已完成',
  cancelled: '已取消',
}

export interface Booking {
  id: ID
  serviceId: ID
  serviceName: string
  artistId: ID
  artistName: string
  date: DateStr
  time: TimeStr
  durationMin: number
  price: Cents
  deposit: Cents
  status: BookingStatus
  occasion?: Occasion
  skinType?: SkinType
  note?: string
  createdAt: Timestamp
  /**
   * 服务端计算，前端据此决定是否显示“取消预约”和“改期”。
   * pending_payment 还没收钱，始终为 true；其他进行中的状态距离开始 24 小时以上才为 true。
   */
  canCancel: boolean
  /** 仅 pending_payment 有：付定金的截止时间（下单后 15 分钟），过了由后端自动取消 */
  payDeadline?: Timestamp
}

export interface CreateBookingReq {
  serviceId: ID
  artistId: ID
  date: DateStr
  time: TimeStr
  occasion?: Occasion
  skinType?: SkinType
  note?: string
}

/** wx.requestPayment 需要的参数，由后端调用微信支付 V3 下单后生成 */
export interface WxPayParams {
  timeStamp: string
  nonceStr: string
  package: string
  signType: 'RSA'
  paySign: string
}

export interface CreateBookingResp {
  booking: Booking
  payment: WxPayParams
}

export interface RescheduleReq {
  artistId: ID
  date: DateStr
  time: TimeStr
}

// ---------- 排班（店主端） ----------

/**
 * 对应排班网格里格子的状态。
 * past：开始时间已过且没有预约的格子（原来空闲或休息都算），不能再设休息或恢复可约，也不计入 stats.free。
 * 已过去但有预约的格子仍按 booked / pending 返回，方便店主查看。
 */
export type SlotState = 'free' | 'booked' | 'pending' | 'blocked' | 'past'

export interface OwnerBookingBrief {
  id: ID
  customerName: string
  serviceName: string
  status: BookingStatus
  /** 需要化妆师提前注意的事项，例如“敏感肌” */
  alert?: string
  note?: string
  /** 服务端计算：pending_confirm 且开始时间还没到才为 true，前端据此决定是否显示“确认” */
  canConfirm: boolean
}

export interface ScheduleCell {
  artistId: ID
  time: TimeStr
  state: SlotState
  booking?: OwnerBookingBrief
}

export interface DaySchedule {
  date: DateStr
  times: TimeStr[]
  artists: Pick<Artist, 'id' | 'name'>[]
  /** 按 time 优先、artist 其次排序，长度 = times.length × artists.length */
  cells: ScheduleCell[]
  stats: {
    total: number
    pending: number
    free: number
    /** pending 里开始时间已过、不能再确认的个数（canConfirm 为 false），是 pending 的子集 */
    stale: number
  }
}

// ---------- 错误 ----------

export type ErrorCode =
  | 'SLOT_TAKEN'       // 时段已被约走
  | 'CANCEL_TOO_LATE'  // 距离开始不足 24 小时
  | 'INVALID_STATE'    // 当前状态不允许该操作
  | 'NOT_FOUND'
  | 'FORBIDDEN'        // 非店主调用店主接口
  | 'UNAUTHORIZED'
  | 'NETWORK'
  | 'UNKNOWN'

export class ApiError extends Error {
  constructor(public code: ErrorCode, message: string) {
    super(message)
    this.name = 'ApiError'
  }
}

/** 给页面用的默认提示文案，页面可以覆盖 */
export const ERROR_TEXT: Record<ErrorCode, string> = {
  SLOT_TAKEN: '这个时间刚被约走了，换一个时间吧',
  CANCEL_TOO_LATE: '距离开始不到 24 小时，需要取消请直接联系门店',
  INVALID_STATE: '这个预约的状态已经变了，刷新看看',
  NOT_FOUND: '没有找到这条信息',
  FORBIDDEN: '只有店主可以进行这个操作',
  UNAUTHORIZED: '登录已过期，请重新进入小程序',
  NETWORK: '网络不太稳定，稍后再试一次',
  UNKNOWN: '出了点问题，稍后再试一次',
}

/** 页面 catch 后用这个取提示文案 */
export function errorText(e: unknown): string {
  return e instanceof ApiError ? (e.message || ERROR_TEXT[e.code]) : ERROR_TEXT.UNKNOWN
}
