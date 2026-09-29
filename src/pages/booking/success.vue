<template>
  <PageLayout>
    <view class="done">
      <view class="done__arch">
        <!-- 对勾描边：按原型路径 M17 29 l7 7 15-16（56 视口），先画短边再画长边 -->
        <view class="check">
          <view class="check__leg check__leg--short" />
          <view class="check__leg check__leg--long" />
        </view>
      </view>

      <view class="done__title">预约好了</view>
      <view class="done__desc">店里确认后会通知你，前一天也会提醒你。</view>
      <view class="done__desc done__desc--last">到时候素颜过来就好。</view>

      <view v-if="status === 'error'" class="ticket ticket--msg">
        <view class="ticket__msg">{{ errorMsg }}</view>
        <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
      </view>

      <view v-else-if="booking" class="ticket">
        <view class="ticket__row"><text class="ticket__label">项目</text>{{ booking.serviceName }}</view>
        <view class="ticket__row"><text class="ticket__label">时间</text>{{ formatDateCN(booking.date) }} {{ booking.time }}</view>
        <view class="ticket__row"><text class="ticket__label">化妆师</text>{{ booking.artistName }}</view>
        <view v-if="shop" class="ticket__row"><text class="ticket__label">地址</text>{{ shop.address }}</view>
        <view class="ticket__row">
          <text class="ticket__label">已付定金</text>
          {{ booking.status === 'pending_payment' ? '支付确认中' : formatPrice(booking.deposit) }}
        </view>
      </view>
    </view>

    <template #footer>
      <view class="actions">
        <AppButton class="actions__btn" @click="switchTab('me')">查看我的预约</AppButton>
        <AppButton class="actions__btn" variant="ghost" @click="switchTab('home')">回到首页</AppButton>
      </view>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { api, errorText, type Booking, type ID, type Shop } from '@/api'
import { formatDateCN } from '@/utils/date'
import { formatPrice } from '@/utils/money'
import { switchTab } from '@/utils/tab'
import AppButton from '@/components/AppButton.vue'
import PageLayout from '@/components/PageLayout.vue'

const booking = ref<Booking>()
const shop = ref<Shop>()
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
let bookingId: ID = ''

async function load() {
  status.value = 'loading'
  try {
    const [b, s] = await Promise.all([api.getBooking(bookingId), api.getShop()])
    booking.value = b
    shop.value = s
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad(query => {
  bookingId = query?.id ?? ''
  load()
})
</script>

<style lang="scss">
.done {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 48rpx 48rpx 40rpx;
  text-align: center;

  &__arch {
    @include arch;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 240rpx;
    height: 296rpx;
    margin: 24rpx 0 44rpx;
    background: $blush;
  }

  &__title {
    margin-bottom: 16rpx;
    font-family: $font-serif;
    font-weight: 600;
    font-size: 48rpx;
  }

  &__desc {
    font-size: $fs-small;
    line-height: 1.7;
    color: $mute;

    &--last {
      margin-bottom: 44rpx;
    }
  }

}

.actions {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
  padding: 20rpx 48rpx 0;
  @include safe-bottom(32rpx);

  &__btn {
    width: 100%;
  }
}

// ---------- 对勾 ----------
// 原型 SVG 为 56×56，×2 换算成 rpx。线宽 3 → 6rpx，圆头。
$check-w: 6rpx;
$short-len: 19.8rpx;  // (17,29)→(24,36)
$long-len: 43.9rpx;   // (24,36)→(39,20)
$draw-total: 0.7s;
$draw-delay: 0.15s;
$short-dur: $draw-total * 0.31; // 按两段长度比例分配时间
$long-dur: $draw-total - $short-dur;

.check {
  position: relative;
  width: 112rpx;
  height: 112rpx;
  border-radius: 50%;
  background: $milk;

  &__leg {
    position: absolute;
    height: $check-w;
    width: $check-w;
    border-radius: $check-w;
    background: $mocha;
    // 以起点圆头的圆心为旋转中心
    transform-origin: ($check-w * 0.5) 50%;
    animation-fill-mode: both;

    &--short {
      left: 34rpx - $check-w * 0.5;
      top: 58rpx - $check-w * 0.5;
      transform: rotate(45deg);
      animation: draw-short $short-dur linear $draw-delay both;
    }

    &--long {
      left: 48rpx - $check-w * 0.5;
      top: 72rpx - $check-w * 0.5;
      transform: rotate(-46.8deg);
      animation: draw-long $long-dur ease-out ($draw-delay + $short-dur) both;
    }
  }
}

@keyframes draw-short {
  from { width: 0; }
  to { width: $short-len + $check-w; }
}

@keyframes draw-long {
  from { width: 0; }
  to { width: $long-len + $check-w; }
}

@media (prefers-reduced-motion: reduce) {
  .check__leg { animation: none; }
  .check__leg--short { width: $short-len + $check-w; }
  .check__leg--long { width: $long-len + $check-w; }
}

// ---------- 预约信息 ----------

.ticket {
  box-sizing: border-box;
  width: 100%;
  padding: 12rpx 36rpx;
  border-radius: 40rpx;
  background: $card;
  text-align: left;

  &--msg {
    padding: 48rpx 36rpx;
    text-align: center;
  }

  &__msg {
    margin-bottom: 28rpx;
    font-size: $fs-small;
    line-height: 1.7;
    color: $mute;
  }

  &__row {
    display: flex;
    justify-content: space-between;
    gap: $gap;
    padding: $gap 0;
    border-bottom: 2rpx dashed $hair;
    font-size: $fs-body;

    &:last-child {
      border-bottom: 0;
    }
  }

  &__label {
    flex: none;
    color: $mute;
  }
}
</style>
