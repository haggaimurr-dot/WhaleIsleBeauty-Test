<template>
  <view class="bottom-bar">
    <view v-if="title || desc || $slots.title" class="bottom-bar__sum">
      <view v-if="title || $slots.title" class="bottom-bar__title">
        <slot name="title">{{ title }}</slot>
      </view>
      <view v-if="desc">{{ desc }}</view>
    </view>
    <slot />
  </view>
</template>

<script setup lang="ts">
/**
 * 页面底部的操作栏，放在 PageLayout 的 #footer 插槽里，自动处理 safe-area。
 * 左边是摘要（title 粗体一行 + desc 小字，第一行需要特殊样式时用 #title 插槽），右边默认插槽放按钮。
 * 例：<BottomBar title="¥298" desc="定金 ¥50，到店付尾款"><AppButton>预约这个妆</AppButton></BottomBar>
 */
defineOptions({ options: { virtualHost: true } })

defineProps<{
  title?: string
  desc?: string
}>()
</script>

<style lang="scss">
.bottom-bar {
  display: flex;
  align-items: center;
  gap: $gap;
  padding: 20rpx 32rpx 0;
  border-top: 2rpx solid $hair;
  background: $card;
  @include safe-bottom(36rpx);

  &__sum {
    flex: 1;
    min-width: 0;
    font-size: $fs-caption + 2rpx;
    line-height: 1.5;
    color: $mute;
  }

  &__title {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
  }
}
</style>
