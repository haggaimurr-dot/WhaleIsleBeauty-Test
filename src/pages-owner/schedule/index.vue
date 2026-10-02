<template>
  <view>
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
        <!-- 预约变动提醒：微信只给一次性订阅，点一次攒一条；确认预约时也会顺手续一条 -->
        <view v-if="quota !== undefined" class="notify" :class="{ 'notify--empty': quota === 0 }">
          <view class="notify__text">
            <view class="notify__title">{{ quota ? `新预约提醒还能收 ${quota} 条` : '新预约提醒用完了' }}</view>
            {{ quota ? '客人付定金、改期、取消都会发到微信' : '续上之前，有新预约微信不会通知你' }}
          </view>
          <AppButton variant="ghost" size="sm" @click="addQuota">{{ quota ? '多收一条' : '续上提醒' }}</AppButton>
        </view>

        <!-- 循环变量别用单字母：uni-app 编译后的数据键也是单字母，会撞上（之前 p 撞了 dateText，提醒条渲染不出来） -->
        <view v-for="pend in pendings" :key="pend.booking.id" class="pending">
          <view class="pending__text">
            <view class="pending__name">新预约：{{ pend.booking.customerName }}</view>
            {{ dateText }} {{ pend.time }}　{{ pend.artistName }}　{{ pend.booking.serviceName }}　定金已付
          </view>
          <AppButton size="sm" :loading="busyKey === pend.booking.id" loading-text="确认中…" @click="confirm(pend.booking)">确认</AppButton>
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

    <!-- 预约详情：点已约 / 待确认的格子打开 -->
    <BottomSheet v-model:open="sheetOpen" @closed="detail = undefined">
      <view v-if="detail" class="detail">
        <view class="detail__head">
          <view>
            <view class="detail__name">{{ detail.booking.customerName }}</view>
            <view class="detail__when">{{ detailWhen }}</view>
          </view>
          <StatusBadge :status="detail.booking.status" />
        </view>

        <view class="detail__rows">
          <view class="detail__row"><text class="detail__k">项目</text>{{ detail.booking.serviceName }}</view>
          <view class="detail__row">
            <text class="detail__k">场合</text>{{ detail.booking.occasion ? OCCASION_LABEL[detail.booking.occasion] : '没选' }}
          </view>
          <view class="detail__row">
            <text class="detail__k">这次肤质</text>{{ detail.booking.skinType ? SKIN_LABEL[detail.booking.skinType] : '没选' }}
          </view>
        </view>

        <view v-if="detail.booking.alert || detail.booking.note" class="detail__alert">
          <view v-if="detail.booking.alert" class="detail__alert-title">{{ detail.booking.alert }}</view>
          <view v-if="detail.booking.note">{{ detail.booking.note }}</view>
        </view>

        <view class="detail__h">肤质档案</view>
        <view v-if="profileRows.length" class="detail__rows">
          <view v-for="row in profileRows" :key="row.k" class="detail__row"><text class="detail__k">{{ row.k }}</text>{{ row.v }}</view>
        </view>
        <view v-else class="detail__empty">
          {{ detail.booking.profile ? '档案里只写了过敏情况，已经放在上面了' : '客人还没填肤质档案，到店时可以当面问一下' }}
        </view>

        <view v-if="detailHint" class="detail__hint">{{ detailHint }}</view>
        <view class="detail__acts">
          <AppButton variant="ghost" class="detail__btn" @click="sheetOpen = false">关闭</AppButton>
          <AppButton
            v-if="detail.booking.canConfirm"
            class="detail__btn"
            :loading="busyKey === detail.booking.id"
            loading-text="确认中…"
            @click="confirmInSheet"
          >
            确认
          </AppButton>
        </view>
      </view>
    </BottomSheet>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  api, errorText, ApiError,
  type DateStr, type DaySchedule, type ID, type OwnerBookingBrief, type ScheduleCell, type TimeStr,
  OCCASION_LABEL, SKIN_LABEL, TONE_LABEL,
} from '@/api'
import { addMinutes, formatDateCN } from '@/utils/date'
import AppButton from '@/components/AppButton.vue'
import BottomSheet from '@/components/BottomSheet.vue'
import StatusBadge from '@/components/StatusBadge.vue'
import DayPicker from '@/components/DayPicker.vue'
import PageLayout from '@/components/PageLayout.vue'
import { OWNER_SUBSCRIBE_READY, requestOwnerSubscribe } from '@/utils/subscribe'

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
      date.value = dates.value.includes(wantDate as DateStr) ? wantDate : dates.value[0]
    }
    loadQuota()
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

// 从提醒消息点进来时带 ?date=，直接看那一天
let wantDate: DateStr | undefined
onLoad(q => { wantDate = q?.date })

// 从“我的”回来或客人刚下了单，都重新拉一次
onShow(load)

// ---------- 预约变动提醒 ----------

/** 还能收几条提醒；模板没配或者没拉到时为 undefined，不显示 */
const quota = ref<number>()

async function loadQuota() {
  if (!OWNER_SUBSCRIBE_READY) return
  try {
    quota.value = (await api.getOwnerNotify()).quota
  } catch {
    // 额度拉不到不影响排班，下次进来再拉
  }
}

/** 必须在点击里同步发起订阅，前面不能有 await */
function subscribeOnce() {
  if (!OWNER_SUBSCRIBE_READY) return Promise.resolve(false)
  return requestOwnerSubscribe().then(async ok => {
    if (!ok) return false
    try {
      quota.value = (await api.addOwnerNotify(1)).quota
      return true
    } catch {
      return false
    }
  })
}

async function addQuota() {
  const ok = await subscribeOnce()
  toast(ok ? '好了，又能多收一条' : '没有同意，这次没加上')
}

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
  if (!busyKey.value) subscribeOnce() // 顺手续一条，不等结果，同不同意都不影响确认
  return run(b.id, async () => {
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
    // 有预约的格子先看详情，确认也在详情里点，避免在网格里误触
    case 'pending':
    case 'booked':
      if (c.booking) openDetail(c.time, c.artistId, c.booking)
  }
}

// ---------- 预约详情 ----------

const sheetOpen = ref(false)
const detail = ref<{ time: TimeStr; artistId: ID; booking: OwnerBookingBrief }>()

function openDetail(time: TimeStr, artistId: ID, booking: OwnerBookingBrief) {
  detail.value = { time, artistId, booking }
  sheetOpen.value = true
}

/** '10月3日 周六 09:00–10:30　小鲸' */
const detailWhen = computed(() => {
  const d = detail.value
  if (!d) return ''
  const end = d.booking.durationMin ? `–${addMinutes(d.time, d.booking.durationMin)}` : ''
  return `${dateText.value} ${d.time}${end}　${artistName(d.artistId)}`
})

/** 过敏情况已经合并在 note 里显示，档案这里不重复 */
const profileRows = computed(() => {
  const p = detail.value?.booking.profile
  if (!p) return []
  return [
    p.skinType && { k: '肤质', v: SKIN_LABEL[p.skinType] },
    p.tone && { k: '肤色', v: TONE_LABEL[p.tone] },
    p.note && { k: '其他', v: p.note },
  ].filter((r): r is { k: string; v: string } => !!r)
})

const detailHint = computed(() => {
  const b = detail.value?.booking
  if (!b) return ''
  if (b.status === 'pending_payment') return '刚下单，还没付定金。15 分钟内没付会自动放出来'
  if (b.status === 'pending_confirm' && !b.canConfirm) return '预约时间已经过了，不能再确认'
  return ''
})

async function confirmInSheet() {
  const b = detail.value?.booking
  if (!b) return
  await confirm(b)
  sheetOpen.value = false
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

.notify {
  display: flex;
  align-items: center;
  gap: $gap;
  margin: 8rpx $page-x 20rpx;
  padding: 24rpx 28rpx;
  border: 1rpx solid $hair;
  border-radius: $r-card;
  background: $card;

  &__text {
    flex: 1;
    min-width: 0;
    font-size: $fs-caption;
    line-height: 1.6;
    color: $mute;
  }

  &__title {
    font-size: $fs-small;
    font-weight: 600;
    color: $ink;
  }

  &--empty &__title {
    color: $mocha;
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

.detail {
  &__head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 20rpx;
    margin-bottom: 28rpx;
  }

  &__name {
    font-family: $font-serif;
    font-weight: 600;
    font-size: $fs-h2 + 2rpx;
  }

  &__when {
    margin-top: 6rpx;
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }

  &__rows {
    padding: 4rpx 28rpx;
    border-radius: $r-inner;
    background: $card;
  }

  &__row {
    display: flex;
    gap: 24rpx;
    padding: 20rpx 0;
    border-bottom: 2rpx dashed $hair;
    font-size: $fs-small;
    line-height: 1.5;

    &:last-child {
      border-bottom: 0;
    }
  }

  &__k {
    flex: none;
    width: 120rpx;
    color: $mute;
  }

  // 过敏、敏感肌这类要化妆师提前准备的，用和格子里一样的裸粉提醒色
  &__alert {
    margin-top: 20rpx;
    padding: 20rpx 28rpx;
    border-radius: $r-inner;
    background: $blush;
    font-size: $fs-small;
    line-height: 1.6;
    color: $blush-ink;
  }

  &__alert-title {
    font-weight: 600;
  }

  &__h {
    margin: 32rpx 0 16rpx;
    font-size: $fs-body;
    font-weight: 600;
  }

  &__empty,
  &__hint {
    font-size: $fs-caption + 2rpx;
    line-height: 1.6;
    color: $mute;
  }

  &__hint {
    margin-top: 28rpx;
  }

  &__acts {
    display: flex;
    gap: 20rpx;
    margin-top: 36rpx;
  }

  &__btn {
    flex: 1;
  }
}
</style>
