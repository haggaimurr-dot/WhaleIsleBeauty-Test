<template>
  <PageLayout ref="layout">
    <template #header>
      <view v-if="me" class="head">
        <ArchImage class="head__avatar" :src="me.avatar" />
        <view>
          <view class="head__name">{{ me.nickname }}</view>
          <view class="head__sub">{{ visitText }}</view>
        </view>
      </view>

      <view class="seg">
        <view class="seg__item" :class="{ 'seg__item--on': tab === 'upcoming' }" @tap="switchSeg('upcoming')">即将到来</view>
        <view class="seg__item" :class="{ 'seg__item--on': tab === 'past' }" @tap="switchSeg('past')">已完成</view>
      </view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="status === 'ok'">
      <!-- 即将到来 -->
      <template v-if="tab === 'upcoming'">
        <view v-if="!upcoming.length" class="panel">
          <view class="panel__text">还没有要来的预约。</view>
          <view class="panel__text panel__text--last">挑一个喜欢的妆，约个时间吧。</view>
          <AppButton @click="switchTab('booking')">去预约</AppButton>
        </view>

        <view v-for="b in upcomingSorted" :key="b.id" class="bcard">
          <view class="bcard__head">
            <view>
              <view class="bcard__when">{{ formatDateCN(b.date) }}</view>
              <view class="bcard__sub">{{ b.time }} 开始，约 {{ b.durationMin }} 分钟</view>
            </view>
            <StatusBadge :status="b.status" />
          </view>

          <view class="bcard__info">
            <ArchImage class="bcard__avatar" :src="artistAvatar(b.artistId)" />
            <view class="bcard__text">
              <view class="bcard__service">{{ b.serviceName }}</view>
              化妆师 {{ b.artistName }}　{{ isUnpaid(b) ? '定金' : '已付定金' }} {{ formatPrice(b.deposit) }}{{ isUnpaid(b) ? ' 还没付' : '' }}
            </view>
          </view>

          <view v-if="isUnpaid(b)" class="bcard__note bcard__note--wait">
            {{ b.payDeadline ? `${formatClock(b.payDeadline)} 前付完定金，这个时段会一直为你留着；过了会自动放出去。` : '付完定金才算约好，时段为你保留 15 分钟。' }}
          </view>
          <view v-else class="bcard__note">
            {{ b.canCancel
              ? '开始前 24 小时以上可以免费改期或取消，定金原路退回。'
              : '距离开始不到 24 小时了，需要改期或取消请直接联系门店。' }}
          </view>

          <view v-if="isUnpaid(b)" class="bcard__acts">
            <AppButton variant="ghost" size="sm" :loading="cancellingId === b.id" loading-text="正在取消…" @click="cancel(b)">
              取消预约
            </AppButton>
            <AppButton size="sm" :loading="payingId === b.id" loading-text="正在付定金…" @click="continuePay(b)">继续付定金</AppButton>
          </view>
          <view v-else class="bcard__acts">
            <template v-if="b.canCancel">
              <AppButton variant="ghost" size="sm" :loading="cancellingId === b.id" loading-text="正在取消…" @click="cancel(b)">
                取消预约
              </AppButton>
              <AppButton variant="ghost" size="sm" @click="reschedule(b)">改期</AppButton>
            </template>
            <AppButton size="sm" @click="navigate">到店导航</AppButton>
          </view>
        </view>
      </template>

      <!-- 已完成 -->
      <template v-else>
        <view v-if="!past.length" class="panel">
          <view class="panel__text">还没有完成的预约，第一次来之后会记在这里。</view>
        </view>

        <view v-for="b in past" :key="b.id" class="past">
          <ArchImage class="past__thumb" shape="rect" :src="serviceCover(b.serviceId)" />
          <view class="past__text">
            <view class="past__service">{{ b.serviceName }}</view>
            {{ formatMonthDay(b.date) }}　化妆师 {{ b.artistName }}
            <StatusBadge v-if="b.status === 'cancelled'" class="past__badge" :status="b.status" />
          </view>
          <AppButton variant="ghost" size="sm" @click="bookAgain(b)">再约一次</AppButton>
        </view>
      </template>
    </template>

    <view class="menu">
      <view class="menu__row" hover-class="menu__row--hover" @tap="openSkinProfile">
        我的肤质档案<text class="menu__hint">{{ me?.skinProfile ?? '还没填写' }}</text>
      </view>
      <view class="menu__row" hover-class="menu__row--hover" @tap="callShop">
        联系门店<text class="menu__hint">{{ shop?.openHours }}</text>
      </view>
      <view v-if="me?.role === 'owner'" class="menu__row" hover-class="menu__row--hover" @tap="openOwner">
        店主工作台<text class="menu__hint">仅店主可见</text>
      </view>
    </view>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import {
  api, payDeposit, errorText, ApiError,
  type Artist, type Booking, type ID, type Me, type Service, type Shop,
} from '@/api'
import { formatClock, formatDateCN, formatMonthDay } from '@/utils/date'
import { formatPrice } from '@/utils/money'
import { requestSubscribe } from '@/utils/subscribe'
import { switchTab } from '@/utils/tab'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import PageLayout from '@/components/PageLayout.vue'
import StatusBadge from '@/components/StatusBadge.vue'

const layout = ref<InstanceType<typeof PageLayout>>()
const tab = ref<'upcoming' | 'past'>('upcoming')

function switchSeg(t: 'upcoming' | 'past') {
  if (t === tab.value) return
  tab.value = t
  layout.value?.scrollToTop()
}

const me = ref<Me>()
const shop = ref<Shop>()
const artists = ref<Artist[]>([])
const services = ref<Service[]>([])
const upcoming = ref<Booking[]>([])
const past = ref<Booking[]>([])
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

/** visitCount 是已完成到店的次数（见 types.ts），“这一次”算下一次 */
const visitText = computed(() => {
  const n = (me.value?.visitCount ?? 0) + 1
  return n === 1 ? '第一次来鲸屿，欢迎' : `这是你第 ${n} 次来鲸屿`
})

const isUnpaid = (b: Booking) => b.status === 'pending_payment'

/** 待付定金的有截止时间，放在最前面；其余保持接口给的时间顺序 */
const upcomingSorted = computed(() => [
  ...upcoming.value.filter(isUnpaid),
  ...upcoming.value.filter(b => !isUnpaid(b)),
])

const artistAvatar = (id: ID) => artists.value.find(a => a.id === id)?.avatar ?? 'placeholder:blank'
const serviceCover = (id: ID) => services.value.find(s => s.id === id)?.cover ?? 'placeholder:blank'

async function load() {
  // 已有数据时静默刷新，不闪空白
  if (status.value !== 'ok') status.value = 'loading'
  try {
    // me 每次都刷新：肤质档案改完回来，摘要要跟着变
    const base = [api.getMe().then(v => { me.value = v })]
    if (!shop.value) {
      base.push(
        api.getShop().then(v => { shop.value = v }),
        api.listArtists().then(v => { artists.value = v }),
        api.listServices().then(v => { services.value = v }),
      )
    }
    const [up, done] = await Promise.all([
      api.listMyBookings('upcoming'),
      api.listMyBookings('past'),
      ...base,
    ])
    upcoming.value = up
    past.value = done
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

// tab 页：每次切回来都刷新，刚约的、店里刚确认的都能看到
onShow(load)

const toast = (title: string) => uni.showToast({ title, icon: 'none', duration: 2500 })

// ---------- 预约操作 ----------

const cancellingId = ref<ID>()

function cancel(b: Booking) {
  const unpaid = isUnpaid(b)
  uni.showModal({
    title: '取消这个预约？',
    content: unpaid ? '还没付定金，取消后这个时段会放给别人。' : `定金 ${formatPrice(b.deposit)} 会原路退回。`,
    confirmText: '取消预约',
    cancelText: '再想想',
    confirmColor: '#6E5446',
    success: async ({ confirm }) => {
      if (!confirm) return
      cancellingId.value = b.id
      try {
        await api.cancelBooking(b.id)
        toast(unpaid ? '已取消预约' : `已取消预约，定金 ${formatPrice(b.deposit)} 将原路退回`)
      } catch (e) {
        if (e instanceof ApiError && e.code === 'CANCEL_TOO_LATE') {
          uni.showModal({ title: '没法在线取消了', content: errorText(e), confirmText: '联系门店', cancelText: '知道了',
            confirmColor: '#6E5446', success: r => { if (r.confirm) callShop() } })
        } else {
          toast(errorText(e))
        }
      } finally {
        cancellingId.value = undefined
        load()
      }
    },
  })
}

const payingId = ref<ID>()

async function continuePay(b: Booking) {
  if (payingId.value) return
  payingId.value = b.id
  try {
    // 和“付定金并预约”是同一个动作，同样请求订阅；已经同意过的再同意一次也不会多发
    await requestSubscribe(['confirmed', 'reminder'])
    const paid = await payDeposit(await api.resumePayment(b.id))
    uni.navigateTo({ url: `/pages/booking/success?id=${paid.id}` })
  } catch (e) {
    toast(errorText(e))
  } finally {
    payingId.value = undefined
    load()
  }
}

const reschedule = (b: Booking) => uni.navigateTo({ url: `/pages/booking/reschedule?rescheduleId=${b.id}` })

const bookAgain = (b: Booking) => switchTab('booking', { serviceId: b.serviceId, artistId: b.artistId })

function navigate() {
  const s = shop.value
  if (!s) return
  uni.openLocation({ latitude: s.latitude, longitude: s.longitude, name: s.name, address: s.address })
}

// ---------- 菜单 ----------

const openSkinProfile = () => uni.navigateTo({ url: '/pages/me/skin' })

function callShop() {
  if (shop.value) uni.makePhoneCall({ phoneNumber: shop.value.phone })
}

const openOwner = () => uni.navigateTo({ url: '/pages-owner/schedule/index' })
</script>

<style lang="scss">
.head {
  display: flex;
  align-items: center;
  gap: 28rpx;
  padding: 8rpx $page-x 0;

  &__avatar {
    flex: none;
    width: 112rpx;
    height: 132rpx;
  }

  &__name {
    font-family: $font-serif;
    font-weight: 600;
    font-size: $fs-h2;
  }

  &__sub {
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }
}

.seg {
  display: flex;
  margin: 44rpx $page-x 28rpx;
  padding: 8rpx;
  border-radius: $r-inner;
  background: $hair;

  &__item {
    flex: 1;
    height: 68rpx;
    line-height: 68rpx;
    border-radius: 20rpx;
    text-align: center;
    font-size: $fs-body;
    color: $mute;

    &--on {
      background: $card;
      color: $ink;
      font-weight: 600;
      box-shadow: 0 4rpx 12rpx -6rpx rgba($mocha, 0.3);
    }
  }
}

.bcard {
  margin: 0 $page-x $gap;
  padding: 32rpx;
  border-radius: 40rpx;
  background: $card;

  &__head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 20rpx;
    margin-bottom: $gap;
  }

  &__when {
    font-family: $font-serif;
    font-weight: 600;
    font-size: $fs-h2;
    line-height: 1.3;
  }

  &__sub {
    margin-top: 8rpx;
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }

  &__info {
    display: flex;
    align-items: center;
    gap: $gap;
    padding: $gap 0;
    border-top: 2rpx dashed $hair;
    border-bottom: 2rpx dashed $hair;
  }

  &__avatar {
    flex: none;
    width: 80rpx;
    height: 96rpx;
  }

  &__text {
    font-size: $fs-small;
    line-height: 1.6;
    color: $mute;
  }

  &__service {
    font-weight: 600;
    color: $ink;
  }

  &__note {
    margin: $gap 0;
    font-size: $fs-caption + 2rpx;
    line-height: 1.6;
    color: $sage-ink;

    // 待付定金不是安全感信息，不用鼠尾草绿
    &--wait {
      color: $blush-ink;
    }
  }

  &__acts {
    display: flex;
    justify-content: flex-end;
    gap: 16rpx;
  }
}

.past {
  display: flex;
  align-items: center;
  gap: $gap;
  margin: 0 $page-x 20rpx;
  padding: $gap;
  border-radius: $r-card;
  background: $card;

  &__thumb {
    flex: none;
    width: 112rpx;
    height: 112rpx;
    border-radius: $r-small;
  }

  &__text {
    flex: 1;
    min-width: 0;
    font-size: $fs-caption + 2rpx;
    line-height: 1.6;
    color: $mute;
  }

  &__service {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
  }

  &__badge {
    margin-left: 12rpx;
  }
}

.panel {
  margin: 0 $page-x $gap;
  padding: 64rpx 40rpx;
  border-radius: 40rpx;
  background: $card;
  text-align: center;

  &__text {
    font-size: $fs-small;
    line-height: 1.7;
    color: $mute;

    &--last {
      margin-bottom: 32rpx;
    }
  }
}

.menu {
  margin: 36rpx $page-x 40rpx;
  border-radius: $r-card;
  background: $card;
  overflow: hidden;

  &__row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 30rpx 32rpx;
    border-bottom: 2rpx solid $hair;
    font-size: $fs-body;

    &:last-child {
      border-bottom: 0;
    }

    &--hover {
      background: $milk;
    }
  }

  &__hint {
    font-size: $fs-caption + 2rpx;
    color: $mute;
  }
}
</style>
