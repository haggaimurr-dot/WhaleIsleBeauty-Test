<template>
  <view>
    <!-- 自定义导航：只有一个返回按钮，和右上角胶囊垂直居中对齐。朋友圈单页模式有微信自己的导航栏，不能返回，不显示 -->
    <view v-if="!singlePage" class="back" :style="backStyle" hover-class="back--hover" @tap="goBack">
      <image class="back__icon" src="/static/icons/back.svg" />
    </view>

    <PageLayout>
      <view class="hero">
        <swiper v-if="service" class="hero__swiper" circular @change="onSwipe">
          <swiper-item v-for="img in service.images" :key="img">
            <ArchImage class="hero__img" shape="rect" :src="img" />
          </swiper-item>
        </swiper>
        <text v-if="service && service.images.length" class="hero__count">
          作品图 {{ imageIndex + 1 }} / {{ service.images.length }}
        </text>
      </view>

      <view v-if="status === 'error'" class="panel">
        <view class="panel__text">{{ errorMsg }}</view>
        <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
      </view>

      <view v-else-if="service" class="body">
        <view class="title">{{ service.name }}</view>
        <view class="meta">约 {{ service.durationMin }} 分钟<text v-if="service.bookedCount">　已有 {{ service.bookedCount }} 位客人选择</text></view>

        <view class="tags">
          <text v-for="t in service.tags" :key="t" class="tag">{{ t }}</text>
        </view>

        <view class="box">
          <view class="box__title">包含</view>
          <view v-for="item in service.includes" :key="item" class="box__li">{{ item }}</view>
        </view>
        <view class="box">
          <view class="box__title">使用的产品</view>
          <view class="box__text">{{ PRODUCTS_TEXT }}</view>
        </view>
        <view class="box">
          <view class="box__title">来之前</view>
          <view class="box__text">{{ BEFORE_TEXT }}</view>
        </view>
      </view>

      <template #footer>
        <!-- 店主下架了：分享出去的链接还能打开，但不能约，引导去看别的项目 -->
        <BottomBar v-if="service?.hidden" title="这个项目暂时不接预约" desc="看看别的项目，或者联系门店问问">
          <AppButton variant="ghost" @click="toHome">看看别的</AppButton>
        </BottomBar>
        <BottomBar v-else-if="service" :desc="`定金 ${formatPrice(service.deposit)}，到店付尾款`">
          <template #title>
            <text class="price">{{ formatPrice(service.price) }}</text>
            <text v-if="service.priceFrom" class="price__from">起</text>
          </template>
          <AppButton @click="toBooking">预约这个妆</AppButton>
        </BottomBar>
      </template>
    </PageLayout>
  </view>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onLoad, onShareAppMessage, onShareTimeline } from '@dcloudio/uni-app'
import { api, errorText, type ID, type Service } from '@/api'
import { formatPrice } from '@/utils/money'
import { isSinglePage, SHOP_NAME, shareImage, sharePath, shareQuery } from '@/utils/share'
import { switchTab } from '@/utils/tab'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'

// 门店通用说明，所有项目一样
const PRODUCTS_TEXT = '底妆和彩妆均为正品专柜品牌，到店时可以查看。如果你有常用的产品，也可以自己带来。'
const BEFORE_TEXT = '素颜到店即可。有过敏史的话，预约时告诉我们，化妆师会提前准备替代产品。'

const service = ref<Service>()
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const imageIndex = ref(0)
let serviceId: ID = ''

// ---------- 返回按钮位置 ----------

const singlePage = isSinglePage()
const menu = uni.getMenuButtonBoundingClientRect()
const BACK_SIZE = 32 // px，与原型一致
const backStyle = {
  top: `${menu.top + (menu.height - BACK_SIZE) / 2}px`,
  width: `${BACK_SIZE}px`,
  height: `${BACK_SIZE}px`,
}

function goBack() {
  // 从分享卡片直接打开时没有上一页，回首页
  if (getCurrentPages().length > 1) uni.navigateBack()
  else switchTab('home')
}

// ---------- 数据 ----------

async function load() {
  status.value = 'loading'
  try {
    service.value = await api.getService(serviceId)
    imageIndex.value = 0
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad(query => {
  serviceId = query?.id ?? ''
  load()
})

// 价格写在标题里：价格透明本来就是卖点。还没加载出来时退回门店名
function shareTitle() {
  const s = service.value
  return s ? `${s.name}｜${formatPrice(s.price, { from: s.priceFrom })}，约 ${s.durationMin} 分钟` : SHOP_NAME
}
onShareAppMessage(() => ({
  title: shareTitle(),
  path: sharePath('/pages/service/detail', { id: serviceId }),
  imageUrl: shareImage(service.value?.cover),
}))
onShareTimeline(() => ({
  title: shareTitle(),
  query: shareQuery({ id: serviceId }),
  imageUrl: shareImage(service.value?.cover),
}))

function onSwipe(e: { detail: { current: number } }) {
  imageIndex.value = e.detail.current
}

const toBooking = () => {
  if (service.value) switchTab('booking', { serviceId: service.value.id })
}

const toHome = () => switchTab('home')
</script>

<style lang="scss">
.back {
  position: fixed;
  left: 32rpx;
  z-index: 10;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 50%;
  background: rgba($card, 0.75);

  &--hover {
    opacity: 0.7;
  }

  &__icon {
    width: 36rpx;
    height: 36rpx;
  }
}

.hero {
  position: relative;
  height: 600rpx;
  overflow: hidden;
  // 底边是一道浅弧，不是拱形
  border-radius: 0 0 50% 50% / 0 0 72rpx 72rpx;
  background: $blush;

  &__swiper,
  &__img {
    width: 100%;
    height: 100%;
  }

  &__count {
    position: absolute;
    left: 0;
    right: 0;
    bottom: 20rpx;
    text-align: center;
    font-size: $fs-caption;
    color: rgba($card, 0.92);
  }
}

.body {
  padding: $page-x;
}

.title {
  margin-bottom: 12rpx;
  font-family: $font-serif;
  font-weight: 600;
  font-size: $fs-title;
}

.meta {
  margin-bottom: 28rpx;
  font-size: $fs-small;
  color: $mute;
}

.tags {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
  margin-bottom: 36rpx;
}

.tag {
  padding: 10rpx 20rpx;
  border-radius: $r-pill;
  background: $blush;
  color: $blush-ink;
  font-size: $fs-caption + 2rpx;
}

.box {
  margin-bottom: $gap;
  padding: 32rpx;
  border-radius: $r-card;
  background: $card;

  &__title {
    margin-bottom: 20rpx;
    font-size: $fs-body;
    font-weight: 600;
  }

  &__li,
  &__text {
    font-size: $fs-small;
    line-height: 1.9;
    color: $ink;
  }

  &__li {
    position: relative;
    padding-left: 36rpx;

    &::before {
      content: '';
      position: absolute;
      left: 8rpx;
      top: 0.95em - 0.2em;
      width: 8rpx;
      height: 8rpx;
      border-radius: 50%;
      background: $mute;
    }
  }

  &__text {
    line-height: 1.8;
  }
}

.price {
  font-family: $font-serif;
  font-weight: 600;
  font-size: 34rpx;
  color: $mocha;

  &__from {
    margin-left: 8rpx;
    font-size: $fs-caption + 2rpx;
    color: $mute;
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
