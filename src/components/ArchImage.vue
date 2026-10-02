<template>
  <view
    class="arch-image"
    :class="{ 'arch-image--arch': shape === 'arch' }"
  >
    <view v-if="placeholder" class="arch-image__fill arch-image__ph" :class="`arch-image__ph--${placeholder}`" />
    <image v-else class="arch-image__fill" :src="src" :mode="mode" />
    <text v-if="label" class="arch-image__label">{{ label }}</text>
  </view>
</template>

<script setup lang="ts">
/**
 * 拱形图片。尺寸由父组件通过 class/style 给定，宽高都要写死，不随屏幕宽度变。
 * src 以 `placeholder:` 开头时显示渐变占位，例如 `placeholder:g2`，对应原型 g1–g6。
 * shape="rect" 时不做拱形，只保留占位能力，圆角由父组件的 class 决定（用于缩略图）。
 */
import { computed } from 'vue'

defineOptions({ options: { virtualHost: true } })

const PLACEHOLDERS = ['g1', 'g2', 'g3', 'g4', 'g5', 'g6']
const PREFIX = 'placeholder:'

const props = withDefaults(defineProps<{
  src: string
  shape?: 'arch' | 'rect'
  /** 图片底部的小字，例如“作品图” */
  label?: string
  mode?: 'aspectFill' | 'aspectFit' | 'widthFix'
}>(), {
  shape: 'arch',
  mode: 'aspectFill',
})

/** 占位的 key；未知的 key 返回 'blank'，显示纯色底 */
const placeholder = computed(() => {
  if (!props.src.startsWith(PREFIX)) return ''
  const key = props.src.slice(PREFIX.length)
  return PLACEHOLDERS.includes(key) ? key : 'blank'
})
</script>

<style lang="scss">
.arch-image {
  position: relative;
  overflow: hidden;

  &--arch {
    border-radius: $r-arch;
  }

  &__fill {
    position: absolute;
    top: 0;
    right: 0;
    bottom: 0;
    left: 0;
    width: 100%;
    height: 100%;
  }

  &__ph {
    background: $blush;
  }

  @each $name, $bg in $placeholders {
    &__ph--#{$name} {
      background: $bg;
    }
  }

  &__label {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 20rpx;
    text-align: center;
    font-size: $fs-caption;
    color: rgba($card, 0.92);
  }
}
</style>
