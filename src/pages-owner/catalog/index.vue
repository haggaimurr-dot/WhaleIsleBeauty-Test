<template>
  <PageLayout>
    <template #header>
      <view class="psub">客人在小程序里看到的门店信息都在这里改。改了马上生效，已经下的单不受影响。</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton v-if="!forbidden" variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="shop">
      <view class="h">门店</view>
      <view class="card" hover-class="card--hover" @tap="openShop">
        <view class="card__main">
          <view class="card__name">{{ shop.name }}</view>
          <view class="card__line">{{ shop.address }}</view>
          <view class="card__line">{{ shop.phone }}　营业 {{ shop.openHours }}</view>
        </view>
        <text class="card__go">修改</text>
      </view>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { api, errorText, ApiError, type Shop } from '@/api'
import AppButton from '@/components/AppButton.vue'
import PageLayout from '@/components/PageLayout.vue'

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const forbidden = ref(false)
const shop = ref<Shop>()

async function load() {
  // 已经有数据时（从编辑页返回）不清空，避免闪一下
  if (!shop.value) status.value = 'loading'
  try {
    // 先确认是店主：资料本身客人也能读，这里提前挡住，免得进到编辑页才报错
    const me = await api.getMe()
    if (me.role !== 'owner') throw new ApiError('FORBIDDEN', '')
    shop.value = await api.getShop()
    status.value = 'ok'
  } catch (e) {
    forbidden.value = e instanceof ApiError && e.code === 'FORBIDDEN'
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onShow(load)

const openShop = () => uni.navigateTo({ url: '/pages-owner/catalog/shop' })
</script>

<style lang="scss">
.psub {
  padding: 8rpx $page-x 0;
  font-size: $fs-caption + 2rpx;
  line-height: 1.7;
  color: $mute;
}

.h {
  margin: 44rpx $page-x 20rpx;
  font-size: 30rpx;
  font-weight: 600;
}

.card {
  display: flex;
  align-items: center;
  gap: $gap;
  margin: 0 $page-x;
  padding: 32rpx;
  border-radius: $r-card;
  background: $card;

  &--hover {
    opacity: 0.7;
  }

  &__main {
    flex: 1;
    min-width: 0;
  }

  &__name {
    margin-bottom: 8rpx;
    font-family: $font-serif;
    font-size: $fs-body + 2rpx;
    font-weight: 600;
    color: $ink;
  }

  &__line {
    font-size: $fs-caption + 2rpx;
    line-height: 1.7;
    color: $mute;
  }

  &__go {
    flex: none;
    font-size: $fs-small;
    color: $mocha;
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
