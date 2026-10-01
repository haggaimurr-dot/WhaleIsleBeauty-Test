/**
 * 资料维护的字段规则。mock、店主端表单和 Go 后端（server/catalog.go）用同一套，改的时候三处一起改。
 *
 * 用法：先 cleanXxx（去掉首尾空格、丢掉空的可选字段和空行），再 validateXxx。
 * validateXxx 返回第一条不符合的提示，全部符合返回 undefined。字数按字算，一个汉字算 1。
 * 引用关系（作品的化妆师、项目是否存在）不在这里查，由 mock / 后端检查。
 */
import type { ArtistInput, ServiceInput, Shop, StyleCategory, WorkInput } from './types'
import { CATEGORY_LABEL } from './types'

export const LIMITS = {
  shopName: 20,
  address: 60,
  phone: 20,
  openHours: 20,

  artistName: 10,
  artistTitle: 10,
  specialty: 12,
  maxYears: 60,

  serviceName: 20,
  summary: 30,
  /** 时长按 15 分钟一档 */
  durationStep: 15,
  minDuration: 30,
  maxDuration: 480,
  /** 价格上限 10 万元（分） */
  maxPrice: 10_000_000,
  includes: 10,
  includeText: 30,
  tags: 5,
  tagText: 10,
  images: 6,

  workTitle: 20,
  durationText: 20,
  minRatio: 0.5,
  maxRatio: 2,

  /** 图片字段（placeholder:gN、以后的 fileID / URL） */
  imageRef: 255,
} as const

const len = (s: string) => [...s].length
const trim = (s?: string) => (s ?? '').trim()
const isInt = (n: unknown): n is number => typeof n === 'number' && Number.isInteger(n)
const isNum = (n: unknown): n is number => typeof n === 'number' && Number.isFinite(n)
const lines = (list?: string[]) => (list ?? []).map(x => trim(x)).filter(Boolean)

/** 必填文本：空着或超长时返回提示 */
function text(v: string, label: string, max: number): string | undefined {
  if (!v) return `${label}还没填`
  if (len(v) > max) return `${label}最多 ${max} 个字`
}

/** 可选文本：只检查长度 */
function optText(v: string | undefined, label: string, max: number): string | undefined {
  if (v && len(v) > max) return `${label}最多 ${max} 个字`
}

function imageRef(v: string, label: string): string | undefined {
  if (!v) return `还没选${label}`
  if (v.length > LIMITS.imageRef) return `${label}地址太长了`
}

const first = (...checks: (string | undefined)[]) => checks.find(Boolean)

// ---------- 门店 ----------

export function cleanShop(s: Shop): Shop {
  return {
    name: trim(s.name), address: trim(s.address), phone: trim(s.phone), openHours: trim(s.openHours),
    latitude: s.latitude, longitude: s.longitude,
  }
}

export function validateShop(s: Shop): string | undefined {
  return first(
    text(s.name, '店名', LIMITS.shopName),
    text(s.address, '地址', LIMITS.address),
    text(s.phone, '电话', LIMITS.phone),
    s.phone && !/^[0-9+\- ]+$/.test(s.phone) ? '电话只能写数字、空格、+ 和 -' : undefined,
    text(s.openHours, '营业时间', LIMITS.openHours),
    !isNum(s.latitude) || !isNum(s.longitude) || Math.abs(s.latitude) > 90 || Math.abs(s.longitude) > 180
      || (s.latitude === 0 && s.longitude === 0)
      ? '地图位置还没选' : undefined,
  )
}

// ---------- 化妆师 ----------

export function cleanArtist(a: ArtistInput): ArtistInput {
  const out: ArtistInput = { name: trim(a.name), years: a.years, avatar: trim(a.avatar) }
  if (trim(a.title)) out.title = trim(a.title)
  if (trim(a.specialty)) out.specialty = trim(a.specialty)
  return out
}

export function validateArtist(a: ArtistInput): string | undefined {
  return first(
    text(a.name, '名字', LIMITS.artistName),
    optText(a.title, '头衔', LIMITS.artistTitle),
    optText(a.specialty, '擅长', LIMITS.specialty),
    !isInt(a.years) || a.years < 0 || a.years > LIMITS.maxYears ? `从业年限填 0–${LIMITS.maxYears} 的整数` : undefined,
    imageRef(a.avatar, '头像'),
  )
}

// ---------- 项目 ----------

const isCategory = (c: unknown): c is StyleCategory => typeof c === 'string' && c in CATEGORY_LABEL

export function cleanService(s: ServiceInput): ServiceInput {
  const images = lines(s.images)
  const out: ServiceInput = {
    name: trim(s.name), summary: trim(s.summary), category: s.category,
    durationMin: s.durationMin, price: s.price, deposit: s.deposit,
    includes: lines(s.includes), tags: lines(s.tags),
    // 封面就是第一张图
    cover: images[0] ?? '', images,
  }
  if (s.priceFrom) out.priceFrom = true
  if (s.hidden) out.hidden = true
  return out
}

export function validateService(s: ServiceInput): string | undefined {
  const { minDuration, maxDuration, durationStep, maxPrice } = LIMITS
  return first(
    text(s.name, '项目名', LIMITS.serviceName),
    text(s.summary, '一句话介绍', LIMITS.summary),
    isCategory(s.category) ? undefined : '还没选风格',
    !isInt(s.durationMin) || s.durationMin < minDuration || s.durationMin > maxDuration || s.durationMin % durationStep
      ? `时长在 ${minDuration}–${maxDuration} 分钟之间，按 ${durationStep} 分钟一档` : undefined,
    !isInt(s.price) || s.price < 100 || s.price > maxPrice ? `价格在 1–${maxPrice / 100} 元之间` : undefined,
    !isInt(s.deposit) || s.deposit < 100 ? '定金至少 1 元' : undefined,
    isInt(s.price) && s.deposit > s.price ? '定金不能比价格高' : undefined,
    s.includes.length > LIMITS.includes ? `包含内容最多 ${LIMITS.includes} 条` : undefined,
    s.includes.find(x => len(x) > LIMITS.includeText) ? `包含内容每条最多 ${LIMITS.includeText} 个字` : undefined,
    s.tags.length > LIMITS.tags ? `标签最多 ${LIMITS.tags} 个` : undefined,
    s.tags.find(x => len(x) > LIMITS.tagText) ? `标签每个最多 ${LIMITS.tagText} 个字` : undefined,
    !s.images.length ? '至少选一张图' : undefined,
    s.images.length > LIMITS.images ? `图片最多 ${LIMITS.images} 张` : undefined,
    s.images.map(x => imageRef(x, '图片')).find(Boolean),
    s.cover !== s.images[0] ? '封面要是第一张图' : undefined,
  )
}

// ---------- 作品 ----------

export function cleanWork(w: WorkInput): WorkInput {
  const out: WorkInput = {
    title: trim(w.title), category: w.category, artistId: w.artistId,
    image: trim(w.image), ratio: w.ratio, durationText: trim(w.durationText),
  }
  if (w.serviceId) out.serviceId = w.serviceId
  return out
}

export function validateWork(w: WorkInput): string | undefined {
  return first(
    text(w.title, '标题', LIMITS.workTitle),
    isCategory(w.category) ? undefined : '还没选风格',
    w.artistId ? undefined : '还没选化妆师',
    imageRef(w.image, '图片'),
    !isNum(w.ratio) || w.ratio < LIMITS.minRatio || w.ratio > LIMITS.maxRatio ? '图片比例不对' : undefined,
    text(w.durationText, '用时', LIMITS.durationText),
  )
}
