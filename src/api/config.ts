/**
 * 真实后端的部署配置，只有 http.ts 用。开通云托管后在这里填环境 ID 和服务名称。
 * - cloud：微信云托管，走 wx.cloud.callContainer（默认）
 * - https：自己的域名，走 uni.request，登录用 code 换 token
 */
export const TRANSPORT = 'cloud' as 'cloud' | 'https'
export const CLOUD_ENV = 'prod-d1gmfnxom1e4a8356' // 云托管环境 ID
export const CLOUD_SERVICE = 'jingyu-api' // 云托管服务名称
export const BASE_URL = 'https://api.example.com' // TODO: 仅 https 模式使用
/** 两种模式路径一致，Go 服务只需要挂在 /v1 下 */
export const API_PREFIX = '/v1'
