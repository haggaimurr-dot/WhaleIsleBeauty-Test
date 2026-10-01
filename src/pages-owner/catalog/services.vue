<template>
  <PageLayout>
    <template #header>
      <view class="psub">客人按这个顺序看到项目。下架的项目客人看不到，已经约了的照常进行。</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <view v-else-if="status === 'ok'" class="list">
      <view
        v-for="svc in services"
        :key="svc.id"
        class="row"
        :class="{ 'row--hidden': svc.hidden }"
        hover-class="row--hover"
        @tap="openEdit(svc.id)"
      >
        <ArchImage class="row__thumb" shape="rect" :src="svc.cover" />
        <view class="row__main">
          <view class="row__name">
            <text class="row__title">{{ svc.name }}</text>
            <text v-if="svc.hidden" class="row__badge">已下架</text>
          </view>
          <view class="row__desc">{{ priceText(svc) }}　约 {{ svc.durationMin }} 分钟</view>
          <view class="row__desc">定金 {{ formatPrice(svc.deposit) }}　{{ CATEGORY_LABEL[svc.category] }}</view>
        </view>
        <text class="row__go">修改</text>
      </view>
    </view>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :desc="summary">
        <AppButton @click="openEdit()">添加项目</AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { api, errorText, CATEGORY_LABEL, type ID, type Service } from '@/api'
import { formatPrice } from '@/utils/money'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const services = ref<Service[]>([])

const priceText = (s: Service) => formatPrice(s.price, { from: s.priceFrom })

const summary = computed(() => {
  const hidden = services.value.filter(s => s.hidden).length
  return `共 ${services.value.length} 个${hidden ? `，${hidden} 个已下架` : ''}`
})

async function load() {
  // 从编辑页返回时不清空，避免闪一下
  if (!services.value.length) status.value = 'loading'
  try {
    services.value = await api.listOwnerServices()
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onShow(load)

const openEdit = (id?: ID) => uni.navigateTo({ url: `/pages-owner/catalog/service${id ? `?id=${id}` : ''}` })
</script>

<style lang="scss">
.psub {
  padding: 8rpx $page-x 0;
  font-size: $fs-caption + 2rpx;
  line-height: 1.7;
  color: $mute;
}

.list {
  margin: 32rpx $page-x 40rpx;
  border-radius: $r-card;
  background: $card;
  overflow: hidden;
}

.row {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 28rpx 32rpx;
  border-bottom: 2rpx solid $hair;

  &:last-child {
    border-bottom: none;
  }

  &--hover {
    background: $milk;
  }

  &--hidden &__thumb,
  &--hidden &__title {
    opacity: 0.5;
  }

  &__thumb {
    flex: none;
    width: 112rpx;
    height: 112rpx;
    border-radius: $r-small;
  }

  &__main {
    flex: 1;
    min-width: 0;
  }

  &__name {
    display: flex;
    align-items: center;
    gap: 12rpx;
    margin-bottom: 6rpx;
  }

  &__title {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
  }

  &__badge {
    flex: none;
    padding: 2rpx 14rpx;
    border-radius: $r-pill;
    background: $hair;
    font-size: 20rpx;
    color: $mute;
  }

  &__desc {
    font-size: $fs-caption + 2rpx;
    line-height: 1.6;
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
