/** 页面只从这里 import：import { api, payDeposit, errorText } from '@/api' */
export { api, USE_MOCK } from './client'
export { payDeposit } from './pay'
export { LIMITS, cleanShop, validateShop, cleanArtist, validateArtist, cleanService, validateService, cleanWork, validateWork } from './catalog'
export type { Api } from './contract'
export * from './types'
