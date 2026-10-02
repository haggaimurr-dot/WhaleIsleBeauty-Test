import type { Api } from './contract'
import { mockApi } from './mock'
import { httpApi } from './http'

/** 演示阶段为 true。接入 Go 后端后改为 false，页面代码不需要改动。 */
export const USE_MOCK = false

export const api: Api = USE_MOCK ? mockApi : httpApi
