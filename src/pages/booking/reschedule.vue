<template>
  <PageLayout>
    <template #header>
      <view v-if="editable" class="chosen">
        <ArchImage class="chosen__thumb" shape="rect" :src="cover" />
        <view class="chosen__text">
          <view class="chosen__name">{{ booking?.serviceName }}</view>
          原来的时间：{{ originalText }}
        </view>
      </view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <!-- 已经不足 24 小时，不能在线改期 -->
    <view v-else-if="booking && !booking.canCancel" class="panel">
      <view class="panel__text">距离开始不到 24 小时，需要改期请直接联系门店</view>
      <AppButton size="sm" @click="callShop">联系门店</AppButton>
    </view>

    <template v-else-if="booking">
      <SlotPicker
        ref="picker"
        v-model:artist-id="artistId"
        v-model:date="date"
        v-model:time="time"
        :service-id="booking.serviceId"
        :artists="artists"
        :dates="dates"
      />

      <view class="pad">
        <view class="policy">改期后需要店里重新确认一次，确认了会通知你。</view>
      </view>
    </template>

    <template #footer>
      <BottomBar v-if="editable" :title="summary" :desc="depositText">
        <AppButton :disabled="!time" :loading="saving" loading-text="正在改期…" @click="submit">确认改期</AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  api, errorText, ApiError,
  type Artist, type Booking, type DateStr, type ID, type Service, type Shop, type TimeStr,
} from '@/api'
import { formatDateCN } from '@/utils/date'
import { formatPrice } from '@/utils/money'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'
import SlotPicker from '@/components/SlotPicker.vue'

let bookingId: ID = ''

const booking = ref<Booking>()
const services = ref<Service[]>([])
const artists = ref<Artist[]>([])
const dates = ref<DateStr[]>([])
const shop = ref<Shop>()
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

const artistId = ref<ID>()
const date = ref<DateStr>()
const time = ref<TimeStr>()
const picker = ref<InstanceType<typeof SlotPicker>>()

const cover = computed(() => services.value.find(s => s.id === booking.value?.serviceId)?.cover ?? 'placeholder:blank')

/** 已加载且还能在线改期 */
const editable = computed(() => status.value === 'ok' && !!booking.value?.canCancel)
const originalText = computed(() => {
  const b = booking.value
  return b ? `${formatDateCN(b.date)} ${b.time}　${b.artistName}` : ''
})
const depositText = computed(() => (booking.value ? `已付定金 ${formatPrice(booking.value.deposit)}，不用再付` : ''))

const summary = computed(() => {
  const artist = artists.value.find(a => a.id === artistId.value)
  return date.value && time.value && artist
    ? `${formatDateCN(date.value)} ${time.value}　${artist.name}`
    : '还没选新时间'
})

async function load() {
  status.value = 'loading'
  try {
    const [b, s, a, d, sh] = await Promise.all([
      api.getBooking(bookingId), api.listServices(), api.listArtists(), api.listBookableDates(), api.getShop(),
    ])
    booking.value = b
    services.value = s
    artists.value = a
    dates.value = d
    shop.value = sh
    // 默认停在原来的化妆师和日期，方便只改时间
    artistId.value = b.artistId
    date.value = d.includes(b.date) ? b.date : d[0]
    time.value = undefined
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad(query => {
  bookingId = query?.rescheduleId ?? ''
  load()
})

const toast = (title: string) => uni.showToast({ title, icon: 'none', duration: 2500 })

function callShop() {
  if (shop.value) uni.makePhoneCall({ phoneNumber: shop.value.phone })
}

// ---------- 提交 ----------

const saving = ref(false)

async function submit() {
  if (!artistId.value || !date.value || !time.value || saving.value) return
  saving.value = true
  try {
    await api.rescheduleBooking(bookingId, { artistId: artistId.value, date: date.value, time: time.value })
    uni.showToast({ title: '改好了，等店里确认', icon: 'none', duration: 1500 })
    setTimeout(() => uni.navigateBack(), 1500)
  } catch (e) {
    const code = e instanceof ApiError ? e.code : undefined
    if (code === 'SLOT_TAKEN') {
      time.value = undefined
      picker.value?.refresh()
      toast(errorText(e))
    } else if (code === 'CANCEL_TOO_LATE') {
      uni.showModal({
        title: '没法在线改期了', content: errorText(e), confirmText: '联系门店', cancelText: '知道了',
        confirmColor: '#6E5446', success: r => { if (r.confirm) callShop() },
      })
    } else {
      toast(errorText(e))
    }
  } finally {
    saving.value = false
  }
}
</script>

<style lang="scss">
.pad {
  padding: 0 $page-x;
}

.chosen {
  display: flex;
  align-items: center;
  gap: $gap;
  margin: 8rpx $page-x 8rpx;
  padding: 20rpx;
  border-radius: 32rpx;
  background: $card;
  font-size: $fs-small;
  color: $mute;

  &__thumb {
    flex: none;
    width: 96rpx;
    height: 96rpx;
    border-radius: $r-small;
  }

  &__text {
    flex: 1;
    min-width: 0;
  }

  &__name {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
  }
}

.policy {
  margin: 44rpx 0 40rpx;
  padding: 24rpx 28rpx;
  border-radius: $r-inner;
  background: $sage-bg;
  font-size: $fs-caption + 2rpx;
  line-height: 1.7;
  color: $sage-ink;
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
