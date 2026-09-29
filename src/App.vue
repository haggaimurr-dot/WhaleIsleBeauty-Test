<script setup lang="ts">
import { onLaunch } from '@dcloudio/uni-app'

onLaunch(() => {})
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
