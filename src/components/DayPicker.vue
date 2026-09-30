<template>
  <scroll-view
    scroll-x
    class="day-picker"
    :show-scrollbar="false"
    enhanced
    scroll-with-animation
    :scroll-into-view="intoView"
  >
    <view
      v-for="d in dates"
      :id="`day-${d}`"
      :key="d"
      class="day-picker__day"
      :class="{ 'day-picker__day--on': d === modelValue }"
      @tap="select(d)"
    >
      <text>{{ dayLabel(d) }}</text>
      <text class="day-picker__num">{{ dayOfMonth(d) }}</text>
    </view>
  </scroll-view>
</template>

<script setup lang="ts">
/**
 * 横向日期选择，用 v-model 绑定选中的日期。
 * 客人端 dates 来自 api.listBookableDates()（从明天开始），店主排班来自 api.listScheduleDates()（从今天开始）。
 * 标签按日期本身算：今天、明天，其余显示周几。
 * 组件自己带页面左右边距，父组件放置时不要再包一层带 padding 的容器。
 */
import { ref, watch } from 'vue'
import type { DateStr } from '@/api'
import { dayLabel, dayOfMonth } from '@/utils/date'

defineOptions({ options: { virtualHost: true } })

const props = defineProps<{
  dates: DateStr[]
  modelValue?: DateStr
}>()

const emit = defineEmits<{ (e: 'update:modelValue', date: DateStr): void }>()

/** 选中的日期不在可视区域时（例如改期带进来的日期）滚过去 */
const intoView = ref('')
watch(
  () => [props.modelValue, props.dates.length] as const,
  ([date]) => { intoView.value = date ? `day-${date}` : '' },
  { immediate: true },
)

function select(d: DateStr) {
  if (d !== props.modelValue) emit('update:modelValue', d)
}
</script>

<style lang="scss">
.day-picker {
  width: 100%;
  white-space: nowrap;
  padding-left: $page-x;
  box-sizing: border-box;

  &__day {
    display: inline-flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 4rpx;
    width: 104rpx;
    height: 128rpx;
    margin-right: 16rpx;
    border-radius: 32rpx;
    background: $card;
    font-size: $fs-caption;
    color: $mute;
    vertical-align: top;

    &:last-child {
      margin-right: $page-x;
    }

    &--on {
      background: $mocha;
      color: $blush;
    }
  }

  &__num {
    font-size: 34rpx;
    font-weight: 600;
    color: $ink;
    font-variant-numeric: tabular-nums;
  }

  &__day--on &__num {
    color: $milk;
  }
}
</style>
