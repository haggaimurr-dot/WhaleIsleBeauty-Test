<template>
  <view>
    <PageLayout ref="layout">
      <template #header>
        <view class="ptitle">作品</view>
        <view class="psub">都是来过的客人，经本人同意后展示</view>
        <scroll-view scroll-x class="filter" :show-scrollbar="false" enhanced>
          <Chip
            v-for="f in FILTERS"
            :key="f.label"
            class="filter__chip"
            :label="f.label"
            :selected="f.category === category"
            @click="selectCategory(f.category)"
          />
        </scroll-view>
      </template>

      <view v-if="status === 'error'" class="panel">
        <view class="panel__text">{{ errorMsg }}</view>
        <AppButton variant="ghost" size="sm" @click="loadWorks">再试一次</AppButton>
      </view>

      <view v-else-if="status === 'ok' && works.length === 0" class="panel">
        <view class="panel__text">这个分类还没有作品，先看看别的吧</view>
        <AppButton variant="ghost" size="sm" @click="selectCategory(undefined)">看全部作品</AppButton>
      </view>

      <view v-else class="grid">
        <view v-for="(col, i) in columns" :key="i" class="grid__col">
          <view
            v-for="w in col"
            :key="w.id"
            class="work"
            hover-class="work--hover"
            @tap="openSheet(w)"
          >
            <ArchImage class="work__img" :src="w.image" :style="{ height: `${workImageHeight(w.ratio)}rpx` }" />
            <view class="work__title">{{ w.title }}</view>
            <view class="work__artist">化妆师 {{ artistName(w.artistId) }}</view>
          </view>
        </view>
      </view>
    </PageLayout>

    <!-- 作品详情弹层 -->
    <view
      class="sheet-mask"
      :class="{ 'sheet-mask--on': sheetOn }"
      @tap="closeSheet()"
      @touchmove.stop.prevent
    />
    <view v-if="current" class="sheet" :class="{ 'sheet--on': sheetOn }" @touchmove.stop.prevent>
      <view class="sheet__grip" />
      <ArchImage class="sheet__img" :src="current.image" label="作品图" />
      <view class="sheet__title">{{ current.title }}</view>
      <view class="sheet__desc">化妆师 {{ artistName(current.artistId) }}　{{ current.durationText }}</view>
      <view class="sheet__desc sheet__desc--last">喜欢这个效果，可以直接约同一位化妆师</view>
      <view class="sheet__row">
        <AppButton variant="ghost" class="sheet__btn" @click="closeSheet()">再看看</AppButton>
        <AppButton class="sheet__btn" @click="bookSame">预约同款</AppButton>
      </view>
    </view>
  </view>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShareAppMessage, onShareTimeline, onShow } from '@dcloudio/uni-app'
import {
  api, errorText, CATEGORY_LABEL,
  type Artist, type ID, type StyleCategory, type Work,
} from '@/api'
import { SHOP_NAME, shareImage, sharePath, shareQuery } from '@/utils/share'
import { switchTab, takeTabParams } from '@/utils/tab'
import { workImageHeight } from '@/utils/work'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import Chip from '@/components/Chip.vue'
import PageLayout from '@/components/PageLayout.vue'

const FILTERS: { label: string; category?: StyleCategory }[] = [
  { label: '全部' },
  ...(Object.keys(CATEGORY_LABEL) as StyleCategory[]).map(c => ({ label: CATEGORY_LABEL[c], category: c })),
]

const layout = ref<InstanceType<typeof PageLayout>>()
const category = ref<StyleCategory>()
const works = ref<Work[]>([])
const artists = ref<Artist[]>([])
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
let loaded = false
let requestSeq = 0

const artistName = (id: ID) => artists.value.find(a => a.id === id)?.name ?? ''

/** 两列瀑布流：每张放进当前较矮的一列。按 rpx 估算，标题和化妆师两行加间距约 110 */
const columns = computed(() => {
  const cols: Work[][] = [[], []]
  const heights = [0, 0]
  for (const w of works.value) {
    const i = heights[0] <= heights[1] ? 0 : 1
    cols[i].push(w)
    heights[i] += workImageHeight(w.ratio) + 110
  }
  return cols
})

async function loadWorks() {
  const seq = ++requestSeq
  status.value = 'loading'
  try {
    const [list, artistList] = await Promise.all([
      api.listWorks(category.value),
      artists.value.length ? Promise.resolve(artists.value) : api.listArtists(),
    ])
    // 连续切换分类时只认最后一次请求
    if (seq !== requestSeq) return
    works.value = list
    artists.value = artistList
    status.value = 'ok'
    openSharedWork()
  } catch (e) {
    if (seq !== requestSeq) return
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

function selectCategory(c?: StyleCategory) {
  if (c === category.value && status.value === 'ok') return
  category.value = c
  layout.value?.scrollToTop()
  loadWorks()
}

// 从分享卡片进来：?category= 定位分类，?workId= 直接打开那件作品
let sharedWorkId: ID | undefined

onLoad(query => {
  const c = query?.category as StyleCategory | undefined
  if (c && c in CATEGORY_LABEL) category.value = c
  sharedWorkId = query?.workId || undefined
})

function openSharedWork() {
  const w = sharedWorkId && works.value.find(x => x.id === sharedWorkId)
  sharedWorkId = undefined
  if (w) openSheet(w)
}

onShow(() => {
  const params = takeTabParams('works')
  if (params) {
    category.value = params.category
    layout.value?.scrollToTop()
    loadWorks()
  } else if (!loaded) {
    loadWorks()
  }
  loaded = true
})

// ---------- 弹层 ----------

const current = ref<Work>()
const sheetOn = ref(false)

// 原生 tabBar 在页面之上，遮罩盖不住它，弹层打开期间先把它藏起来
const noop = () => {}
let tabBarTimer: ReturnType<typeof setTimeout> | undefined

function openSheet(w: Work) {
  clearTimeout(tabBarTimer)
  current.value = w
  uni.hideTabBar({ animation: false, fail: noop })
  // 先渲染到屏幕外，下一帧再加 --on，transform 过渡才会生效
  setTimeout(() => { sheetOn.value = true }, 20)
}

/** 等弹层滑出（0.3s）再放出 tabBar，避免页面高度变化时弹层跳一下；离开页面时立即放出 */
function closeSheet(immediate = false) {
  sheetOn.value = false
  clearTimeout(tabBarTimer)
  tabBarTimer = setTimeout(() => uni.showTabBar({ animation: false, fail: noop }), immediate ? 0 : 300)
}

// 弹层开着就分享这件作品，否则分享当前分类。发给朋友和朋友圈内容一样，朋友圈只带 query
function shareContent() {
  const w = sheetOn.value ? current.value : undefined
  if (w) {
    return {
      title: `${w.title}｜化妆师 ${artistName(w.artistId)}`,
      query: { category: w.category, workId: w.id },
      imageUrl: shareImage(w.image),
    }
  }
  const c = category.value
  return {
    title: c ? `${SHOP_NAME}的${CATEGORY_LABEL[c]}妆作品` : `${SHOP_NAME}的作品，都是来过的客人`,
    query: { category: c },
  }
}

onShareAppMessage(() => {
  const { query, ...rest } = shareContent()
  return { ...rest, path: sharePath('/pages/works/index', query) }
})
onShareTimeline(() => {
  const { query, ...rest } = shareContent()
  return { ...rest, query: shareQuery(query) }
})

function bookSame() {
  const w = current.value
  if (!w) return
  closeSheet(true)
  switchTab('booking', { artistId: w.artistId, serviceId: w.serviceId })
}
</script>

<style lang="scss">
.ptitle {
  margin: 8rpx 0;
  padding: 0 $page-x;
  font-family: $font-serif;
  font-weight: 600;
  font-size: $fs-title;
}

.psub {
  margin-bottom: 28rpx;
  padding: 0 $page-x;
  font-size: $fs-caption + 2rpx;
  color: $mute;
}

.filter {
  white-space: nowrap;
  padding: 0 0 28rpx $page-x;
  box-sizing: border-box;

  &__chip {
    margin-right: 16rpx;

    &:last-child {
      margin-right: $page-x;
    }
  }
}

.grid {
  display: flex;
  gap: $gap;
  padding: 0 32rpx 32rpx;

  &__col {
    flex: 1;
    min-width: 0;
  }
}

.work {
  margin-bottom: 32rpx;

  &--hover {
    opacity: 0.85;
  }

  &__img {
    margin-bottom: 16rpx;
  }

  &__title {
    font-size: $fs-small;
    font-weight: 600;
  }

  &__artist {
    font-size: $fs-caption;
    color: $mute;
  }
}

.panel {
  margin: 20rpx $page-x;
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

.sheet-mask {
  position: fixed;
  top: 0;
  right: 0;
  bottom: 0;
  left: 0;
  z-index: 30;
  background: rgba($ink, 0.35);
  opacity: 0;
  pointer-events: none;
  transition: opacity 0.25s;

  &--on {
    opacity: 1;
    pointer-events: auto;
  }
}

.sheet {
  position: fixed;
  left: 0;
  right: 0;
  bottom: 0;
  z-index: 31;
  padding: 28rpx 40rpx 0;
  border-radius: 56rpx 56rpx 0 0;
  background: $milk;
  transform: translateY(105%);
  transition: transform 0.3s cubic-bezier(0.2, 0.8, 0.2, 1);
  @include safe-bottom(44rpx);

  &--on {
    transform: none;
  }

  &__grip {
    width: 72rpx;
    height: 8rpx;
    margin: 0 auto 28rpx;
    border-radius: 4rpx;
    background: $hair;
  }

  &__img {
    width: 400rpx;
    height: 520rpx;
    margin: 0 auto 32rpx;
  }

  &__title {
    margin-bottom: 8rpx;
    text-align: center;
    font-family: $font-serif;
    font-weight: 600;
    font-size: 38rpx;
  }

  &__desc {
    text-align: center;
    font-size: $fs-small;
    line-height: 1.7;
    color: $mute;

    &--last {
      margin-bottom: 36rpx;
    }
  }

  &__row {
    display: flex;
    gap: 20rpx;
  }

  &__btn {
    flex: 1;
  }
}
</style>
