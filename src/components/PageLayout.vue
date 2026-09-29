<template>
  <view class="page-layout">
    <view v-if="$slots.header" class="page-layout__header">
      <slot name="header" />
    </view>
    <scroll-view
      class="page-layout__body"
      scroll-y
      enhanced
      :show-scrollbar="false"
      enable-back-to-top
      :scroll-into-view="intoView"
    >
      <view id="page-layout-top" />
      <slot />
    </scroll-view>
    <view v-if="$slots.footer" class="page-layout__footer">
      <slot name="footer" />
    </view>
  </view>
</template>

<script setup lang="ts">
/**
 * 页面骨架：头尾固定，只有中间内容滚动，并且不显示滚动条。所有页面都用它。
 * 页面本身在 pages.json 里设了 disableScroll，不会整页滚动。
 * - #header：固定在顶部（页面标题、筛选、分段等）
 * - 默认插槽：可滚动的内容
 * - #footer：固定在底部（BottomBar 或操作按钮）
 */
import { nextTick, ref } from 'vue'

defineOptions({ options: { virtualHost: true } })

const intoView = ref('')

/** 切换筛选、分段、日期后调用，内容回到顶部 */
function scrollToTop() {
  intoView.value = ''
  nextTick(() => { intoView.value = 'page-layout-top' })
}

defineExpose({ scrollToTop })
</script>

<style lang="scss">
.page-layout {
  display: flex;
  flex-direction: column;
  height: 100vh;
  overflow: hidden;

  &__header,
  &__footer {
    flex: none;
  }

  &__body {
    flex: 1;
    height: 0;
  }
}
</style>
