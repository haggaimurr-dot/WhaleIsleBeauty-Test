<template>
  <PageLayout>
    <template #header>
      <view class="psub">客人预约时按这个顺序选化妆师，排班表也是这些人。</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <view v-else-if="status === 'ok'" class="list">
      <view v-for="artist in artists" :key="artist.id" class="row" hover-class="row--hover" @tap="openEdit(artist.id)">
        <ArchImage class="row__avatar" :src="artist.avatar" />
        <view class="row__main">
          <view class="row__name">{{ artist.name }}</view>
          <view class="row__desc">{{ artistDesc(artist) }}</view>
        </view>
        <text class="row__go">修改</text>
      </view>
    </view>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :desc="`共 ${artists.length} 位`">
        <AppButton @click="openEdit()">添加化妆师</AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { onShow } from '@dcloudio/uni-app'
import { api, errorText, type Artist, type ID } from '@/api'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const artists = ref<Artist[]>([])

/** 和客人端选化妆师时的写法一致（SlotPicker） */
const artistDesc = (a: Artist) => [a.title, `${a.years} 年`, a.specialty].filter(Boolean).join(' ')

async function load() {
  // 从编辑页返回时不清空，避免闪一下
  if (!artists.value.length) status.value = 'loading'
  try {
    artists.value = await api.listArtists()
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onShow(load)

const openEdit = (id?: ID) => uni.navigateTo({ url: `/pages-owner/catalog/artist${id ? `?id=${id}` : ''}` })
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

  &__avatar {
    flex: none;
    width: 80rpx;
    height: 96rpx;
  }

  &__main {
    flex: 1;
    min-width: 0;
  }

  &__name {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
  }

  &__desc {
    margin-top: 4rpx;
    font-size: $fs-caption + 2rpx;
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
