<template>
  <view class="slot-picker">
    <view class="slot-picker__h">化妆师</view>
    <view class="slot-picker__artists">
      <view
        v-for="a in artists"
        :key="a.id"
        class="artist"
        :class="{ 'artist--on': a.id === artistId }"
        @tap="selectArtist(a.id)"
      >
        <ArchImage class="artist__avatar" :src="a.avatar" />
        <view>{{ a.name }}</view>
        <view class="artist__desc">{{ artistDesc(a) }}</view>
      </view>
    </view>

    <view class="slot-picker__h">日期</view>
    <DayPicker :model-value="date" :dates="dates" @update:model-value="selectDate" />

    <view class="slot-picker__h">时间</view>
    <view v-if="slotsError" class="slot-picker__msg">
      {{ slotsError }}
      <text class="slot-picker__retry" @tap="refresh">再试一次</text>
    </view>
    <view v-else-if="!slots.length" class="slot-picker__msg">正在看哪些时间还空着…</view>
    <view v-else class="slot-picker__slots">
      <view
        v-for="s in slots"
        :key="s.time"
        class="slot"
        :class="{ 'slot--off': !s.available, 'slot--on': s.time === time }"
        @tap="selectTime(s)"
      >
        {{ s.time }}
      </view>
    </view>
    <view class="slot-picker__hint">划线的时段已经约满</view>
  </view>
</template>

<script setup lang="ts">
/**
 * 选化妆师、日期、时间。预约页和改期页共用。
 * 可约时段由组件自己按 (serviceId, artistId, date) 拉取；换化妆师或日期时清空已选时间。
 * 用法：<SlotPicker ref="picker" :service-id v-model:artist-id v-model:date v-model:time :artists :dates />
 */
import { ref, watch } from 'vue'
import { api, errorText, type Artist, type DateStr, type ID, type SlotView, type TimeStr } from '@/api'
import ArchImage from './ArchImage.vue'
import DayPicker from './DayPicker.vue'

defineOptions({ options: { virtualHost: true } })

const props = defineProps<{
  serviceId?: ID
  artists: Artist[]
  dates: DateStr[]
  artistId?: ID
  date?: DateStr
  time?: TimeStr
}>()

const emit = defineEmits<{
  (e: 'update:artistId', id: ID): void
  (e: 'update:date', date: DateStr): void
  (e: 'update:time', time: TimeStr | undefined): void
}>()

/** '主理人 8 年'、'6 年 擅长新娘' */
const artistDesc = (a: Artist) => [a.title, `${a.years} 年`, a.specialty].filter(Boolean).join(' ')

function selectArtist(id: ID) {
  if (id === props.artistId) return
  emit('update:artistId', id)
  emit('update:time', undefined)
}

function selectDate(d: DateStr) {
  emit('update:date', d)
  emit('update:time', undefined)
}

function selectTime(s: SlotView) {
  if (s.available) emit('update:time', s.time)
}

// ---------- 可约时段 ----------

const slots = ref<SlotView[]>([])
const slotsError = ref('')
let seq = 0

async function refresh() {
  const { serviceId, artistId, date } = props
  if (!serviceId || !artistId || !date) return
  const mine = ++seq
  slots.value = []
  slotsError.value = ''
  try {
    const list = await api.listSlots(artistId, date, serviceId)
    if (mine !== seq) return
    slots.value = list
    // 已选的时间刚被约走了，清掉
    if (props.time && !list.some(s => s.time === props.time && s.available)) emit('update:time', undefined)
  } catch (e) {
    if (mine !== seq) return
    slotsError.value = errorText(e)
  }
}

watch(() => [props.serviceId, props.artistId, props.date], refresh, { immediate: true })

/** 提交时遇到 SLOT_TAKEN，父组件调用它重新拉取 */
defineExpose({ refresh })
</script>

<style lang="scss">
.slot-picker {
  &__h {
    margin: 44rpx $page-x 24rpx;
    font-size: 30rpx;
    font-weight: 600;
  }

  &__artists {
    display: flex;
    gap: 20rpx;
    padding: 0 $page-x;
  }

  &__slots {
    display: flex;
    flex-wrap: wrap;
    gap: 16rpx;
    padding: 0 $page-x;
  }

  &__msg,
  &__hint {
    padding: 0 $page-x;
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }

  &__msg {
    line-height: 80rpx;
  }

  &__retry {
    margin-left: 16rpx;
    color: $mocha;
  }

  &__hint {
    margin-top: 16rpx;
  }
}

.artist {
  flex: 1;
  min-width: 0;
  padding: 24rpx 16rpx;
  border-radius: $r-card;
  background: $card;
  text-align: center;
  font-size: $fs-small;

  &--on {
    box-shadow: inset 0 0 0 3rpx $mocha;
  }

  &__avatar {
    width: 112rpx;
    height: 136rpx;
    margin: 0 auto 16rpx;
  }

  &__desc {
    margin-top: 4rpx;
    font-size: $fs-caption;
    line-height: 1.4;
    color: $mute;
  }
}

.slot {
  // 4 列，3 个间隔
  width: calc((100% - 48rpx) / 4);
  height: 80rpx;
  line-height: 80rpx;
  border-radius: $r-small;
  background: $card;
  text-align: center;
  font-size: $fs-body;
  font-variant-numeric: tabular-nums;

  &--off {
    background: transparent;
    color: $disabled;
    text-decoration: line-through;
    box-shadow: inset 0 0 0 2rpx $hair;
  }

  &--on {
    background: $blush;
    color: $mocha;
    font-weight: 600;
    box-shadow: inset 0 0 0 3rpx $rose;
  }
}
</style>
