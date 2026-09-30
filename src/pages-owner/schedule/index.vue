<template>
  <PageLayout ref="layout">
    <template #header>
      <view class="ohead">
        <text class="ohead__title">排班</text>
        <text class="ohead__date">{{ dateText }}</text>
      </view>

      <template v-if="!forbidden">
        <view class="stats">
          <view class="stats__item"><text class="stats__num">{{ schedule?.stats.total ?? '–' }}</text>当天预约</view>
          <view class="stats__item">
            <text class="stats__num">{{ schedule ? schedule.stats.pending - schedule.stats.stale : '–' }}</text>待确认
            <text v-if="schedule?.stats.stale" class="stats__extra">另有 {{ schedule.stats.stale }} 个已过时</text>
          </view>
          <view class="stats__item"><text class="stats__num">{{ schedule?.stats.free ?? '–' }}</text>空闲时段</view>
        </view>
        <view class="days">
          <DayPicker :model-value="date" :dates="dates" @update:model-value="changeDate" />
        </view>
      </template>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton v-if="!forbidden" variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else>
      <view v-for="p in pendings" :key="p.booking.id" class="pending">
        <view class="pending__text">
          <view class="pending__name">新预约：{{ p.booking.customerName }}</view>
          {{ dateText }} {{ p.time }}　{{ p.artistName }}　{{ p.booking.serviceName }}　定金已付
        </view>
        <AppButton size="sm" :loading="busyKey === p.booking.id" loading-text="确认中…" @click="confirm(p.booking)">确认</AppButton>
      </view>

      <view v-if="schedule" class="grid">
        <view class="grid__row">
          <view class="grid__time" />
          <view v-for="a in schedule.artists" :key="a.id" class="grid__head">{{ a.name }}</view>
        </view>
        <view v-for="row in rows" :key="row.time" class="grid__row">
          <view class="grid__time">{{ row.time }}</view>
          <view
            v-for="c in row.cells"
            :key="c.artistId"
            class="cell"
            :class="[`cell--${cellTone(c)}`, { 'cell--busy': busyKey === cellKey(c) }]"
            @tap="tapCell(c)"
          >
            <template v-if="c.state === 'free'">可约</template>
            <template v-else-if="c.state === 'blocked'">休息</template>
            <template v-else-if="c.state === 'past'">已过</template>
            <template v-else-if="c.booking">
              <text>{{ c.booking.customerName }}</text>
              <text class="cell__sub" :class="{ 'cell__sub--alert': c.state === 'booked' && c.booking.alert && c.booking.status !== 'pending_payment' }">
                {{ cellSub(c) }}
              </text>
            </template>
          </view>
        </view>
      </view>

      <view class="legend">
        <view class="legend__item"><text class="legend__dot legend__dot--booked" />已约</view>
        <view class="legend__item"><text class="legend__dot legend__dot--pending" />待确认</view>
        <view class="legend__item"><text class="legend__dot legend__dot--free" />空闲，点一下设为休息</view>
        <view class="legend__item"><text class="legend__dot legend__dot--blocked" />休息</view>
        <view v-if="hasPast" class="legend__item"><text class="legend__dot legend__dot--past" />已过去，不能再改</view>
      </view>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  api, errorText, ApiError,
  type DateStr, type DaySchedule, type ID, type OwnerBookingBrief, type ScheduleCell,
} from '@/api'
import { formatDateCN } from '@/utils/date'
import AppButton from '@/components/AppButton.vue'
import DayPicker from '@/components/DayPicker.vue'
import PageLayout from '@/components/PageLayout.vue'

const layout = ref<InstanceType<typeof PageLayout>>()
const dates = ref<DateStr[]>([])
const date = ref<DateStr>()
const schedule = ref<DaySchedule>()
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const forbidden = ref(false)
let seq = 0

/** 本次在工作台里确认过的预约，格子用“已确认”的绿色标出来，和其他已约区分 */
const confirmedHere = ref(new Set<ID>())

const dateText = computed(() => (date.value ? formatDateCN(date.value) : ''))

const artistName = (id: ID) => schedule.value?.artists.find(a => a.id === id)?.name ?? ''
const cellKey = (c: ScheduleCell) => c.booking?.id ?? `${c.artistId}|${c.time}`

/** cells 按 time 优先、artist 其次排序，转成一行一个时间 */
const rows = computed(() => {
  const s = schedule.value
  if (!s) return []
  const n = s.artists.length
  return s.times.map((time, i) => ({ time, cells: s.cells.slice(i * n, i * n + n) }))
})

const hasPast = computed(() => !!schedule.value?.cells.some(c => c.state === 'past'))

const pendings = computed(() =>
  (schedule.value?.cells ?? [])
    .filter((c): c is ScheduleCell & { booking: OwnerBookingBrief } => c.state === 'pending' && !!c.booking?.canConfirm)
    .map(c => ({ time: c.time, artistName: artistName(c.artistId), booking: c.booking })),
)

function cellTone(c: ScheduleCell) {
  if (c.state === 'booked' && c.booking && confirmedHere.value.has(c.booking.id)) return 'ok'
  return c.state
}

function cellSub(c: ScheduleCell) {
  const b = c.booking
  if (!b) return ''
  if (c.state === 'pending') return b.canConfirm ? '待确认' : '已过时，未确认'
  if (b.status === 'pending_payment') return '待付定金'
  if (cellTone(c) === 'ok') return '已确认'
  return b.alert ?? b.serviceName
}

// ---------- 数据 ----------

async function load() {
  const mine = ++seq
  if (!schedule.value) status.value = 'loading'
  try {
    if (!dates.value.length) {
      dates.value = await api.listScheduleDates()
      date.value = dates.value[0]
    }
    if (!date.value) return
    const s = await api.getDaySchedule(date.value)
    if (mine !== seq) return
    schedule.value = s
    status.value = 'ok'
  } catch (e) {
    if (mine !== seq) return
    forbidden.value = e instanceof ApiError && e.code === 'FORBIDDEN'
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

// 从“我的”回来或客人刚下了单，都重新拉一次
onShow(load)

function changeDate(d: DateStr) {
  date.value = d
  schedule.value = undefined
  layout.value?.scrollToTop()
  load()
}

// ---------- 操作 ----------

const busyKey = ref<string>()
const toast = (title: string) => uni.showToast({ title, icon: 'none', duration: 2000 })

async function run(key: string, action: () => Promise<unknown>, done: string) {
  if (busyKey.value) return
  busyKey.value = key
  try {
    await action()
    toast(done)
  } catch (e) {
    toast(errorText(e))
  } finally {
    busyKey.value = undefined
    await load()
  }
}

function confirm(b: OwnerBookingBrief) {
  run(b.id, async () => {
    await api.confirmBooking(b.id)
    confirmedHere.value.add(b.id)
  }, '已确认，已通知客人')
}

function tapCell(c: ScheduleCell) {
  const d = date.value
  if (!d) return
  const who = `${artistName(c.artistId)} ${c.time}`
  switch (c.state) {
    case 'free':
      return run(cellKey(c), () => api.blockSlot(c.artistId, d, c.time), `${who} 设为休息`)
    case 'blocked':
      return run(cellKey(c), () => api.unblockSlot(c.artistId, d, c.time), `${who} 恢复可约`)
    case 'past':
      return toast('这个时段已经过去了')
    case 'pending':
      if (!c.booking) return
      return c.booking.canConfirm ? confirm(c.booking) : toast('预约时间已经过了，不能再确认')
    case 'booked': {
      const b = c.booking
      if (!b) return
      if (b.status === 'pending_payment') return toast(`${b.customerName} 刚下单，还没付定金。15 分钟内没付会自动放出来`)
      const extra = [b.alert, b.note && `备注：${b.note}`].filter(Boolean).join('，')
      return toast([b.customerName, b.serviceName, extra].filter(Boolean).join('　'))
    }
  }
}
</script>

<style lang="scss">
.days {
  padding: 24rpx 0 20rpx;
}

.ohead {
  display: flex;
  justify-content: space-between;
  align-items: flex-end;
  padding: 8rpx $page-x 0;

  &__title {
    font-family: $font-serif;
    font-weight: 600;
    font-size: $fs-title;
  }

  &__date {
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }
}

.stats {
  display: flex;
  gap: 16rpx;
  margin: 28rpx $page-x 0;

  &__item {
    flex: 1;
    padding: 20rpx $gap;
    border-radius: $r-inner;
    background: $card;
    font-size: $fs-caption;
    color: $mute;
  }

  &__extra {
    display: block;
    font-size: 20rpx;
    color: $disabled;
  }

  &__num {
    display: block;
    font-size: 40rpx;
    font-weight: 600;
    color: $ink;
    font-variant-numeric: tabular-nums;
  }
}

.pending {
  display: flex;
  align-items: center;
  gap: $gap;
  margin: 8rpx $page-x 20rpx;
  padding: 28rpx;
  border-radius: $r-card;
  background: $blush;

  &__text {
    flex: 1;
    min-width: 0;
    font-size: $fs-caption + 2rpx;
    line-height: 1.6;
    color: $blush-ink;
  }

  &__name {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
  }
}

.grid {
  margin: 8rpx $gap 0;

  &__row {
    display: flex;
    gap: 10rpx;
    margin-bottom: 10rpx;
  }

  &__time {
    flex: none;
    width: 76rpx;
    padding-top: 12rpx;
    font-size: $fs-caption;
    color: $mute;
    font-variant-numeric: tabular-nums;
  }

  &__head {
    flex: 1;
    padding: 8rpx 0 2rpx;
    text-align: center;
    font-size: $fs-caption + 2rpx;
    font-weight: 600;
  }
}

.cell {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  justify-content: center;
  min-height: 100rpx;
  padding: 12rpx;
  box-sizing: border-box;
  border-radius: 20rpx;
  font-size: $fs-caption;
  line-height: 1.35;
  color: $ink;

  &__sub {
    font-size: 20rpx;
    color: $mute;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;

    &--alert {
      color: $blush-ink;
      font-weight: 600;
    }
  }

  &--free {
    align-items: center;
    color: $disabled;
    box-shadow: inset 0 0 0 2rpx $hair;
  }

  &--booked {
    background: $hair;
  }

  &--pending {
    background: $card;
    box-shadow: inset 0 0 0 3rpx $rose;

    .cell__sub {
      color: $blush-ink;
    }
  }

  &--ok {
    background: $sage-bg;

    .cell__sub {
      color: $sage-ink;
    }
  }

  &--blocked {
    align-items: center;
    color: $mute;
    background: repeating-linear-gradient(135deg, $milk 0 12rpx, $hair 12rpx 14rpx);
  }

  &--past {
    align-items: center;
    color: $disabled;
  }

  &--busy {
    opacity: 0.5;
  }
}

.legend {
  display: flex;
  flex-wrap: wrap;
  gap: 12rpx 28rpx;
  margin: $gap $page-x 0;
  font-size: $fs-caption;
  @include safe-bottom(36rpx);
  color: $mute;

  &__item {
    display: flex;
    align-items: center;
  }

  &__dot {
    display: inline-block;
    width: 20rpx;
    height: 20rpx;
    margin-right: 8rpx;
    border-radius: 6rpx;

    &--booked { background: $hair; }
    &--pending { background: $card; box-shadow: inset 0 0 0 3rpx $rose; }
    &--free { box-shadow: inset 0 0 0 2rpx $hair; }
    &--blocked { background: repeating-linear-gradient(135deg, $milk 0 6rpx, $hair 6rpx 8rpx); }
    &--past { border: 2rpx dashed $disabled; box-sizing: border-box; }
  }
}

.panel {
  margin: 40rpx $page-x;
  padding: 64rpx 40rpx;
  border-radius: 40rpx;
  background: $card;
  text-align: center;

  &__text {
    margin-bottom: 32rpx;
    font-size: $fs-small;
    line-height: 1.7;
    color: $mute;
  }
}
</style>
