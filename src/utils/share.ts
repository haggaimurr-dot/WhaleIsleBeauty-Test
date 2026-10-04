/**
 * 分享卡片用的小工具。三个可分享页面：首页、作品、项目详情，都能发给朋友和分享到朋友圈。
 * 标题语气和页面文案一致：像化妆师在介绍，不写“限时”“特惠”。
 */

export const SHOP_NAME = '鲸屿美妆'

/**
 * 卡片封面：只用真实图片地址。
 * 演示阶段都是 placeholder: 渐变，返回 undefined，微信会截取当前页面顶部当封面。
 */
export const shareImage = (src?: string) => (src && !src.startsWith('placeholder:') ? src : undefined)

/** 拼分享路径，undefined 的参数不带 */
export function sharePath(page: string, query: Record<string, string | undefined> = {}) {
  const qs = Object.entries(query)
    .filter(([, v]) => v !== undefined && v !== '')
    .map(([k, v]) => `${k}=${encodeURIComponent(v as string)}`)
    .join('&')
  return qs ? `${page}?${qs}` : page
}

/**
 * 是不是从朋友圈打开的“单页模式”（场景值 1154）。
 * 单页模式没有登录态、不能跳页面、没有 tabBar；页面里点了跳转，微信会提示“前往小程序”。
 */
export function isSinglePage() {
  try {
    return uni.getLaunchOptionsSync().scene === 1154
  } catch {
    return false
  }
}

/** 朋友圈的 query 不带页面路径，和 sharePath 一样去掉空参数 */
export const shareQuery = (query: Record<string, string | undefined>) => sharePath('', query).replace(/^\?/, '')
