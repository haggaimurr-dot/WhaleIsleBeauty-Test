<template>
  <PageLayout>
    <template #header>
      <view class="psub">客人在“作品”里按这个顺序看到，新加的排在最前。</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <view v-else-if="status === 'ok' && !works.length" class="panel">
      <view class="panel__text">还没有作品。放几张来过的客人的妆面，客人看了更放心。</view>
    </view>

    <view v-else-if="status === 'ok'" class="list">
      <view v-for="w in works" :key="w.id" class="row" hover-class="row--hover" @tap="openEdit(w.id)">
        <ArchImage class="row__thumb" :src="w.image" />
        <view class="row__main">
          <view class="row__name">
            <text class="row__title">{{ w.title }}</text>
            <text class="row__badge">{{ CATEGORY_LABEL[w.category] }}</text>
          </view>
          <view class="row__desc">化妆师 {{ artistName(w.artistId) }}　{{ w.durationText }}</view>
          <view class="row__desc">{{ serviceText(w) }}</view>
        </view>
        <text class="row__go">修改</text>
      </view>
    </view>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :desc="works.length ? `共 ${works.length} 件` : ''">
        <AppButton @click="openEdit()">添加作品</AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { api, errorText, CATEGORY_LABEL, type Artist, type ID, type Service, type Work } from '@/api'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const works = ref<Work[]>([])
const artists = ref<Artist[]>([])
const services = ref<Service[]>([])

const artistName = (id: ID) => artists.value.find(a => a.id === id)?.name ?? ''

/** 客人点“预约同款”时会不会预选项目：没关联、或关联的项目下架了都不会 */
function serviceText(w: Work) {
  const s = w.serviceId && services.value.find(x => x.id === w.serviceId)
  if (!s) return '没关联项目'
  return s.hidden ? `同款项目 ${s.name} 已下架` : `同款项目 ${s.name}`
}

async function load() {
  // 从编辑页返回时不清空，避免闪一下
  if (status.value !== 'ok') status.value = 'loading'
  try {
    ;[works.value, artists.value, services.value] = await Promise.all([
      api.listOwnerWorks(), api.listArtists(), api.listOwnerServices(),
    ])
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onShow(load)

const openEdit = (id?: ID) => uni.navigateTo({ url: `/pages-owner/catalog/work${id ? `?id=${id}` : ''}` })
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

  &__thumb {
    flex: none;
    width: 96rpx;
    height: 120rpx;
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
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
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
    font-size: $fs-small;
    line-height: 1.7;
    color: $mute;
  }
}
</style>
