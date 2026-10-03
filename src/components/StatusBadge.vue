<template>
  <text class="status-badge" :class="`status-badge--${TONE[status]}`">{{ label || STATUS_LABEL[status] }}</text>
</template>

<script setup lang="ts">
/**
 * 预约状态标签，文案来自 STATUS_LABEL。
 * 鼠尾草绿只给“店里已确认”；等待中的用裸粉；已结束的用灰。
 */
import { STATUS_LABEL, type BookingStatus } from '@/api'

defineOptions({ options: { virtualHost: true } })

/** label：需要换个说法时用，例如店主端把没来的预约写成“没来”，颜色仍按 status */
defineProps<{ status: BookingStatus; label?: string }>()

const TONE: Record<BookingStatus, 'wait' | 'ok' | 'past'> = {
  pending_payment: 'wait',
  pending_confirm: 'wait',
  confirmed: 'ok',
  completed: 'past',
  cancelled: 'past',
}
</script>

<style lang="scss">
.status-badge {
  display: inline-block;
  padding: 6rpx 16rpx;
  border-radius: $r-pill;
  font-size: $fs-caption;
  font-weight: 500;
  line-height: 1.4;
  white-space: nowrap;

  &--wait {
    background: $blush;
    color: $blush-ink;
  }

  &--ok {
    background: $sage-bg;
    color: $sage-ink;
  }

  &--past {
    background: $hair;
    color: $mute;
  }
}
</style>
