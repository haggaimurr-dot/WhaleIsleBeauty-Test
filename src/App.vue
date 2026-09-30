<script setup lang="ts">
import { onLaunch } from '@dcloudio/uni-app'
import { api } from '@/api'

// 启动时先登录。页面不用等它：请求发出前会自己等这次登录；这里失败了，第一次请求时会再登录一次
onLaunch(() => {
  api.login().catch(() => {})
})
</script>

<style lang="scss">
page {
  background-color: $milk;
  color: $ink;
  font-family: $font-sans;
  font-size: $fs-body;
}

// 不显示任何滚动条（scroll-view 另外设置了 show-scrollbar=false）
::-webkit-scrollbar {
  display: none;
  width: 0;
  height: 0;
  color: transparent;
}
</style>
