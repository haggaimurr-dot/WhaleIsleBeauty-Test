<template>
  <view v-if="visible" class="bottom-sheet">
    <view
      class="bottom-sheet__mask"
      :class="{ 'bottom-sheet__mask--on': on }"
      @tap="close"
      @touchmove.stop.prevent
    />
    <view class="bottom-sheet__panel" :class="{ 'bottom-sheet__panel--on': on }">
      <view class="bottom-sheet__grip" />
      <slot />
    </view>
  </view>
</template>

<script setup lang="ts">
/**
 * 底部弹层：遮罩 + 从底部滑入的面板，点遮罩关闭。用 v-model:open 控制。
 * 动效规则里允许的两处动画之一；关闭动画结束后才卸载内容，并发出 closed。
 * 注意：原生 tabBar 在页面之上，tabBar 页里用时要自己藏起 tabBar（见作品页）。
 * 例：<BottomSheet v-model:open="sheetOpen"><view>内容</view></BottomSheet>
 */
import { ref, watch } from 'vue'

defineOptions({ options: { virtualHost: true } })

const props = defineProps<{ open: boolean }>()
const emit = defineEmits<{
  (e: 'update:open', open: boolean): void
  (e: 'closed'): void
}>()

/** visible 控制挂载，on 控制过渡；两者错开，进出场动画才会生效 */
const visible = ref(false)
const on = ref(false)
let timer: ReturnType<typeof setTimeout> | undefined

watch(() => props.open, open => {
  clearTimeout(timer)
  if (open) {
    visible.value = true
    // 先渲染到屏幕外，下一帧再加 --on
    timer = setTimeout(() => { on.value = true }, 20)
  } else if (visible.value) {
    on.value = false
    timer = setTimeout(() => {
      visible.value = false
      emit('closed')
    }, 300)
  }
}, { immediate: true })

function close() {
  emit('update:open', false)
}
</script>

<style lang="scss">
.bottom-sheet {
  &__mask {
    position: fixed;
    top: 0;
    right: 0;
    bottom: 0;
    left: 0;
    z-index: 30;
    background: rgba($ink, 0.35);
    opacity: 0;
    transition: opacity 0.25s;

    &--on {
      opacity: 1;
    }
  }

  &__panel {
    position: fixed;
    left: 0;
    right: 0;
    bottom: 0;
    z-index: 31;
    max-height: 85vh;
    overflow: hidden;
    padding: 28rpx 40rpx 0;
    box-sizing: border-box;
    border-radius: 56rpx 56rpx 0 0;
    background: $milk;
    transform: translateY(105%);
    transition: transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
    @include safe-bottom(44rpx);

    &--on {
      transform: none;
    }
  }

  &__grip {
    width: 72rpx;
    height: 8rpx;
    margin: 0 auto 28rpx;
    border-radius: 4rpx;
    background: $hair;
  }
}
</style>
