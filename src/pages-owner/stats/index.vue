<template>
  <PageLayout>
    <template #header>
      <view class="ohead">
        <text class="ohead__title">{{ current ? monthTitle(current.month) : '经营统计' }}</text>
        <text class="ohead__note">{{ headNote }}</text>
      </view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton v-if="!forbidden" variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <view v-else-if="status === 'loading'" class="panel">
      <view class="panel__text">正在算这几个月的数…</view>
    </view>

    <template v-else-if="current">
      <view class="grid">
        <view class="tile">
          <view class="tile__label">预约</view>
          <view class="tile__num">{{ current.bookings }}<text class="tile__unit">单</text></view>
          <view class="tile__sub">{{ bookingDelta }}</view>
        </view>
        <view class="tile">
          <view class="tile__label">定金收入</view>
          <view class="tile__num">{{ formatPrice(current.deposit) }}</view>
          <view class="tile__sub">取消的已退回，不算在内</view>
        </view>
        <view class="tile">
          <view class="tile__label">取消率</view>
          <view class="tile__num">{{ cancelRate(current) }}</view>
          <view class="tile__sub">{{ cancelSub }}</view>
        </view>
        <view class="tile">
          <view class="tile__label">回头客</view>
          <view class="tile__num">{{ current.returning }}<text class="tile__unit">位</text></view>
          <view class="tile__sub">{{ current.customers ? `共 ${current.customers} 位客人，占 ${percent(current.returning, current.customers)}` : '这个月还没有客人' }}</view>
        </view>
      </view>

      <view class="h">近 6 个月<text class="h__hint">点一行看那个月</text></view>
      <view class="table">
        <view class="table__row table__row--head">
          <text class="table__month">月份</text>
          <text class="table__cell">预约</text>
          <text class="table__cell table__cell--wide">定金</text>
          <text class="table__cell">取消率</text>
          <text class="table__cell">回头客</text>
        </view>
        <view
          v-for="row in months"
          :key="row.month"
          class="table__row"
          :class="{ 'table__row--on': row.month === selected }"
          @tap="selected = row.month"
        >
          <text class="table__month">{{ monthShort(row.month) }}</text>
          <text class="table__cell">{{ row.bookings }}</text>
          <text class="table__cell table__cell--wide">{{ formatPrice(row.deposit) }}</text>
          <text class="table__cell">{{ cancelRate(row) }}</text>
          <text class="table__cell">{{ row.returning }}</text>
        </view>
      </view>

      <view class="foot">
        按到店日期算到月份，只算付过定金的预约。回头客是这个月来之前已经到店做过一次的客人。
      </view>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { api, errorText, ApiError, type MonthStats } from '@/api'
import { todayStr } from '@/utils/date'
import { formatPrice } from '@/utils/money'
import AppButton from '@/components/AppButton.vue'
import PageLayout from '@/components/PageLayout.vue'

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const forbidden = ref(false)
/** 本月在前 */
const months = ref<MonthStats[]>([])
const selected = ref('')

const current = computed(() => months.value.find(m => m.month === selected.value))
const previous = computed(() => months.value[months.value.findIndex(m => m.month === selected.value) + 1])
const thisMonth = () => todayStr().slice(0, 7)

const headNote = computed(() => {
  if (!current.value) return ''
  return current.value.month === thisMonth() ? '本月 · 含已经约了还没到的' : current.value.month.slice(0, 4) + ' 年'
})

const bookingDelta = computed(() => {
  const cur = current.value
  const prev = previous.value
  if (!cur || !prev) return '再往前没有对比'
  const d = cur.bookings - prev.bookings
  return d > 0 ? `比上月多 ${d} 单` : d < 0 ? `比上月少 ${-d} 单` : '和上月一样'
})

/** 没来的算在取消里，有的话单独点出来；旧后端没有 noShow 字段时按 0 */
const cancelSub = computed(() => {
  const cur = current.value
  if (!cur?.cancelled) return '没有付了定金又取消的'
  return cur.noShow ? `取消 ${cur.cancelled} 单，其中 ${cur.noShow} 单没来` : `付了定金又取消 ${cur.cancelled} 单`
})

const monthTitle = (month: string) => `${Number(month.slice(5))} 月`
/** 表格里：本月写“本月”，跨年的写上年份 */
function monthShort(month: string) {
  if (month === thisMonth()) return '本月'
  const m = `${Number(month.slice(5))}月`
  return month.slice(0, 4) === thisMonth().slice(0, 4) ? m : `${month.slice(2, 4)}年${m}`
}

const percent = (n: number, total: number) => `${Math.round((n / total) * 100)}%`
/** 取消 / (预约 + 取消)；一单都没有时显示横杠 */
const cancelRate = (m: MonthStats) => {
  const total = m.bookings + m.cancelled
  return total ? percent(m.cancelled, total) : '–'
}

async function load() {
  if (!months.value.length) status.value = 'loading'
  try {
    months.value = await api.listMonthStats()
    if (!months.value.some(m => m.month === selected.value)) selected.value = months.value[0]?.month ?? ''
    status.value = 'ok'
  } catch (e) {
    forbidden.value = e instanceof ApiError && e.code === 'FORBIDDEN'
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onShow(load)
</script>

<style lang="scss">
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

  &__note {
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }
}

.grid {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
  margin: 28rpx $page-x 0;
}

.tile {
  box-sizing: border-box;
  width: calc(50% - 8rpx);
  padding: 28rpx $gap;
  border-radius: $r-inner;
  background: $card;

  &__label {
    font-size: $fs-caption;
    color: $mute;
  }

  &__num {
    margin: 8rpx 0 6rpx;
    font-family: $font-serif;
    font-size: 48rpx;
    font-weight: 600;
    line-height: 1.3;
    color: $ink;
  }

  &__unit {
    margin-left: 6rpx;
    font-family: $font-sans;
    font-size: $fs-caption;
    font-weight: 400;
    color: $mute;
  }

  &__sub {
    font-size: 20rpx;
    line-height: 1.5;
    color: $mute;
  }
}

.h {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  margin: 48rpx $page-x 20rpx;
  font-size: 30rpx;
  font-weight: 600;

  &__hint {
    font-size: $fs-caption;
    font-weight: 400;
    color: $mute;
  }
}

.table {
  margin: 0 $page-x;
  padding: 8rpx 0;
  border-radius: $r-card;
  background: $card;
  overflow: hidden;

  &__row {
    display: flex;
    align-items: center;
    margin: 0 12rpx;
    padding: 22rpx 20rpx;
    border-radius: $r-small;
    font-size: $fs-small;
    color: $ink;

    &--head {
      padding-top: 18rpx;
      padding-bottom: 12rpx;
      font-size: $fs-caption;
      color: $mute;
    }

    &--on {
      background: $blush;
      color: $blush-ink;
    }
  }

  &__month {
    flex: 1.2;
  }

  &__cell {
    flex: 1;
    text-align: right;
    font-variant-numeric: tabular-nums;

    &--wide {
      flex: 1.6;
    }
  }
}

.foot {
  margin: 28rpx $page-x 48rpx;
  font-size: $fs-caption;
  line-height: 1.7;
  color: $mute;
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
