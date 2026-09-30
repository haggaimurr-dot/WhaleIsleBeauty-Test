import { fileURLToPath, URL } from 'node:url'
import { defineConfig } from 'vitest/config'

// 只测纯 TS 逻辑（mock、请求层、工具函数），不经过 uni-app 的 vite 插件
export default defineConfig({
  resolve: {
    alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
  },
  test: {
    include: ['src/**/*.test.ts'],
    environment: 'node',
  },
})
