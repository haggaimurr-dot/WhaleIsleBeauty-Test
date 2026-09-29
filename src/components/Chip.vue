<template>
  <view class="chip" :class="{ 'chip--on': selected }" @tap="emit('click')">
    <slot>{{ label }}</slot>
  </view>
</template>

<script setup lang="ts">
/** 可单选的标签，用于筛选和预约表单。选中状态由父组件控制。 */
defineOptions({ options: { virtualHost: true } })

defineProps<{
  label?: string
  selected?: boolean
}>()

const emit = defineEmits<{ (e: 'click'): void }>()
</script>

<style lang="scss">
.chip {
  display: inline-block;
  padding: 14rpx 28rpx;
  border-radius: $r-pill;
  background: $card;
  color: $ink;
  font-size: $fs-small;
  line-height: 1.4;
  white-space: nowrap;

  &--on {
    background: $blush;
    color: $mocha;
    box-shadow: inset 0 0 0 2rpx $rose;
  }
}
</style>
