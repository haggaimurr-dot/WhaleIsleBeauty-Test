<template>
  <view
    class="app-btn"
    :class="[`app-btn--${variant}`, `app-btn--${size}`, { 'app-btn--disabled': disabled, 'app-btn--loading': loading }]"
    :hover-class="inactive ? 'none' : 'app-btn--hover'"
    @tap="onTap"
  >
    <template v-if="loading && loadingText">{{ loadingText }}</template>
    <slot v-else />
  </view>
</template>

<script setup lang="ts">
/**
 * 按钮。primary 是摩卡色实心胶囊，ghost 是白底细边框。一个页面底部最多一个 primary。
 * 宽度默认随文字，需要撑满时由父组件的 class 给 flex: 1 或 width。
 * loading 时不响应点击；按动效规则不加转圈，用 loadingText 说明正在做什么，例如“正在预约…”。
 */
import { computed } from 'vue'

defineOptions({ options: { virtualHost: true } })

const props = withDefaults(defineProps<{
  variant?: 'primary' | 'ghost'
  size?: 'md' | 'sm'
  disabled?: boolean
  loading?: boolean
  loadingText?: string
}>(), {
  variant: 'primary',
  size: 'md',
})

const emit = defineEmits<{ (e: 'click'): void }>()

const inactive = computed(() => props.disabled || props.loading)

function onTap() {
  if (!inactive.value) emit('click')
}
</script>

<style lang="scss">
.app-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  box-sizing: border-box;
  border-radius: $r-pill;
  font-weight: 500;
  white-space: nowrap;

  &--md {
    height: 88rpx;
    padding: 0 44rpx;
    font-size: 30rpx;
  }

  &--sm {
    height: 64rpx;
    padding: 0 28rpx;
    font-size: $fs-small;
  }

  &--primary {
    background: $mocha;
    color: $milk;
  }

  &--ghost {
    background: $card;
    color: $mocha;
    box-shadow: inset 0 0 0 2rpx $hair;
  }

  &--primary.app-btn--disabled {
    background: $disabled;
  }

  &--ghost.app-btn--disabled {
    color: $disabled;
  }

  &--loading {
    opacity: 0.7;
  }

  &--hover {
    opacity: 0.85;
  }
}
</style>
