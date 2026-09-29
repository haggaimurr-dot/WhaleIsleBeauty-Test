import { defineConfig } from "vite";
import uni from "@dcloudio/vite-plugin-uni";

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [uni()],
  css: {
    preprocessorOptions: {
      scss: {
        // Vite 5.2 仍走 Sass 旧版 JS API；uni-app 把 uni.scss（含 @import）注入每个样式块。
        // 两者都只是弃用提示，不影响产物，这里静音避免刷屏。
        silenceDeprecations: ["legacy-js-api", "import"],
      },
    },
  },
});
