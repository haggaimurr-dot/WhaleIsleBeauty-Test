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
  /**
   * 已下架：客人看不到、不能新约，已经约了的照常进行。
   * 只有店主接口和 getService 会返回下架的项目（详情页据此显示“暂时不接预约”）
   */
  hidden?: boolean
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

// ---------- 资料维护（店主端） ----------

/** 新建、修改化妆师时提交的内容，整份替换 */
export type ArtistInput = Omit<Artist, 'id'>

/** 新建、修改项目时提交的内容，整份替换。bookedCount 由服务端维护，新项目从 0 开始 */
export type ServiceInput = Omit<Service, 'id' | 'bookedCount'>

/** 新建、修改作品时提交的内容，整份替换 */
export type WorkInput = Omit<Work, 'id'>

// ---------- 用户 ----------

export type Role = 'customer' | 'owner'

export interface Me {
  id: ID
  /** 微信不再提供昵称：首次登录时后端随机生成，例如“柚子27”（一个温和的小词 + 两位数字），之后不变 */
  nickname: string
  /** 同上，首次登录时从 placeholder:g1–g6 里随机选一个，ArchImage 画成拱形 */
  avatar: string
  role: Role
  /**
   * 已到店完成的次数：该用户状态为 completed 的预约数。
   * 不含已取消、待付定金和还没到的预约。前端显示“第 visitCount + 1 次来”。
   */
  visitCount: number
  /**
   * 肤质档案的一句话摘要，例如“敏感肌 · 冷白皮”，由服务端根据 SkinProfile 生成。
   * 没填过或全部清空时不返回。完整内容用 getSkinProfile 取
   */
  skinProfile?: string
}

// ---------- 肤质档案 ----------

export type SkinTone = 'cool_fair' | 'warm_fair' | 'natural' | 'wheat' | 'unsure'

export const TONE_LABEL: Record<SkinTone, string> = {
  cool_fair: '冷白皮', warm_fair: '暖白皮', natural: '自然色', wheat: '小麦色', unsure: '不确定',
}

/** 客人自己填写，每次预约时化妆师都能提前看到。所有字段都可以不填 */
export interface SkinProfile {
  skinType?: SkinType
  tone?: SkinTone
  /** 过敏或不能用的成分、产品，例如“对酒精过敏”，不超过 200 字 */
  allergies?: string
  /** 其他想让化妆师知道的，例如“单眼皮”“戴隐形眼镜”，不超过 200 字 */
  note?: string
  /** 服务端写入；从没保存过时不返回 */
  updatedAt?: Timestamp
}

/** 整份替换：不带的字段、空字符串都表示清空 */
export type UpdateSkinProfileReq = Omit<SkinProfile, 'updatedAt'>

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
 * pending_confirm 到了开始时间店里还没确认，由后端自动取消，定金原路退回
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
  /** 仅 cancelled 有，前端据此说明为什么取消了 */
  cancelReason?: CancelReason
}

/**
 * customer：客人自己取消（含店里代客人取消）
 * pay_timeout：超过 15 分钟没付定金
 * not_confirmed：到了开始时间店里还没确认，定金已原路退回
 */
export type CancelReason = 'customer' | 'pay_timeout' | 'not_confirmed'

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

/**
 * 店主的预约变动提醒（订阅消息）。微信只给一次性订阅：店主每同意一次，后端才能发一条。
 * quota 是还能发几条，后端每发一条减一；微信说没订阅（43101）时清零。
 */
export interface OwnerNotifyStatus {
  quota: number
}

/** 发给店主的提醒种类：付完定金的新预约、客人改期、客人取消（都只针对付过定金的预约） */
export type OwnerNoticeKind = 'new' | 'rescheduled' | 'cancelled'

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
  // ---- 以下用于店主点开格子看详情 ----
  durationMin?: number
  /** 客人这次选的场合 */
  occasion?: Occasion
  /** 客人这次选的肤质（可能和档案不同） */
  skinType?: SkinType
  /** 客人的肤质档案，下单时的最新版本；没填过不返回。过敏情况已经合并在 note 里 */
  profile?: SkinProfile
}

/**
 * 客人已下单但还没付定金（booking.status 为 pending_payment）的格子返回 booked：
 * 时段在付款截止前为客人保留，店主不能设休息；它不计入 stats.total，超时后后端自动放出。
 */
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

// ---------- 经营统计（店主端） ----------

/**
 * 一个月的汇总。按预约的到店日期（date）归到月份，本月也算上已经约了、还没到的。
 * 只统计付过定金的预约：没付就取消、付款超时的不算，还在待付定金的也不算。
 */
export interface MonthStats {
  /** 'YYYY-MM' */
  month: string
  /** 付过定金、没有取消的预约数（待确认、已确认、已完成） */
  bookings: number
  /** 付过定金后取消的（客人取消、店里没确认自动取消）。取消率 = cancelled / (bookings + cancelled)，前端算 */
  cancelled: number
  /** 定金收入：bookings 里这些预约的定金合计。取消的定金都已原路退回，不算 */
  deposit: Cents
  /** bookings 里有几位不同的客人 */
  customers: number
  /** customers 里的回头客：在这个月最后一次预约的日期之前，已经到店完成过至少一次 */
  returning: number
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
