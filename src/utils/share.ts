/**
 * 分享卡片用的小工具。三个可分享页面：首页、作品、项目详情。
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
