<template>
  <PageLayout>
    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="status === 'ok'">
      <!-- 和“作品”页瀑布流里的一格一样大，高度、标题一眼能看到 -->
      <view class="preview">
        <view class="preview__cell">
          <ArchImage :src="form.image || 'placeholder:none'" :style="{ height: `${workImageHeight(form.ratio)}rpx` }" :label="form.image ? '' : '还没选图'" />
          <view class="preview__title">{{ form.title.trim() || '标题' }}</view>
          <view class="preview__artist">化妆师 {{ artistName || '…' }}</view>
        </view>
        <view class="preview__note">客人在“作品”里看到的样子</view>
      </view>

      <view class="h">图片</view>
      <view class="pics">
        <view
          v-for="src in PICS"
          :key="src"
          class="pics__item"
          :class="{ 'pics__item--on': form.image === src }"
          @tap="form.image = src"
        >
          <ArchImage class="pics__img" :src="src" />
        </view>
      </view>
      <view class="hint">演示版先从这几个里选，以后可以上传照片。</view>

      <view class="h">高度<text class="h__opt">高一点的图在瀑布流里更显眼</text></view>
      <view class="chips">
        <Chip v-for="o in RATIOS" :key="o.label" :label="o.label" :selected="workImageHeight(form.ratio) === workImageHeight(o.ratio)" @click="form.ratio = o.ratio" />
      </view>

      <view class="h">标题</view>
      <view class="pad">
        <input v-model="form.title" class="field" placeholder="比如 氧气上镜妆" placeholder-class="field__ph" :maxlength="LIMITS.workTitle" />
      </view>

      <view class="h">风格</view>
      <view class="chips">
        <Chip v-for="c in CATEGORIES" :key="c" :label="CATEGORY_LABEL[c]" :selected="form.category === c" @click="form.category = c" />
      </view>

      <view class="h">化妆师</view>
      <view class="chips">
        <Chip v-for="a in artists" :key="a.id" :label="a.name" :selected="form.artistId === a.id" @click="form.artistId = a.id" />
      </view>

      <view class="h">同款项目<text class="h__opt">可以不选</text></view>
      <view class="chips">
        <Chip label="不关联" :selected="!form.serviceId" @click="form.serviceId = ''" />
        <Chip
          v-for="s in services"
          :key="s.id"
          :label="s.hidden ? `${s.name}（已下架）` : s.name"
          :selected="form.serviceId === s.id"
          @click="pickService(s)"
        />
      </view>
      <view class="hint">{{ serviceHint }}</view>

      <view class="h">用时</view>
      <view class="pad">
        <input v-model="form.durationText" class="field" placeholder="比如 约 90 分钟、需提前沟通" placeholder-class="field__ph" :maxlength="LIMITS.durationText" />
      </view>

      <view class="pad">
        <view class="policy">放上来的照片要先问过客人本人，同意了再放。客人想撤下，删掉就好。</view>
      </view>

      <view v-if="id" class="remove">
        <AppButton variant="ghost" size="sm" :loading="removing" loading-text="正在删除…" @click="remove">删除这件作品</AppButton>
      </view>
      <view v-else class="gap" />
    </template>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :desc="id ? '改了马上生效' : '添加后排在“作品”最前面'">
        <AppButton :disabled="!dirty" :loading="saving" loading-text="正在保存…" @click="save">
          {{ id ? '保存修改' : '添加作品' }}
        </AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  api, errorText, cleanWork, validateWork, LIMITS, CATEGORY_LABEL,
  type Artist, type ID, type Service, type StyleCategory, type Work, type WorkInput,
} from '@/api'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import Chip from '@/components/Chip.vue'
import PageLayout from '@/components/PageLayout.vue'
import { workImageHeight } from '@/utils/work'

const PICS = ['g1', 'g2', 'g3', 'g4', 'g5', 'g6'].map(g => `placeholder:${g}`)
const CATEGORIES = Object.keys(CATEGORY_LABEL) as StyleCategory[]
/** 瀑布流里只有这几档高度（utils/work.ts），以后上传照片时按照片宽高落到最近一档 */
const RATIOS = [
  { label: '矮', ratio: 1 },
  { label: '适中', ratio: 1.25 },
  { label: '较高', ratio: 1.33 },
  { label: '最高', ratio: 1.5 },
]

const id = ref<ID>('')
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')
const artists = ref<Artist[]>([])
const services = ref<Service[]>([])

/** 可选项用空字符串表示没选，提交时再转 */
interface Form {
  title: string; category?: StyleCategory; artistId: ID; serviceId: ID
  image: string; ratio: number; durationText: string
}
const EMPTY: Form = { title: '', category: undefined, artistId: '', serviceId: '', image: '', ratio: 1.25, durationText: '' }
const form = reactive<Form>({ ...EMPTY })
const saved = ref<Form>({ ...EMPTY })

function toInput(f: Form): WorkInput {
  return cleanWork({
    title: f.title, category: f.category as StyleCategory, artistId: f.artistId,
    serviceId: f.serviceId || undefined, image: f.image, ratio: f.ratio, durationText: f.durationText,
  })
}

function toForm(w: Work): Form {
  return {
    title: w.title, category: w.category, artistId: w.artistId, serviceId: w.serviceId ?? '',
    image: w.image, ratio: w.ratio, durationText: w.durationText,
  }
}

const dirty = computed(() => JSON.stringify(toInput(form)) !== JSON.stringify(toInput(saved.value)))

const artistName = computed(() => artists.value.find(a => a.id === form.artistId)?.name ?? '')


// ---------- 同款项目 ----------

/** 选了项目时，风格和用时还空着就顺手带上 */
function pickService(s: Service) {
  form.serviceId = s.id
  if (!form.category) form.category = s.category
  if (!form.durationText.trim()) form.durationText = `约 ${s.durationMin} 分钟`
}

const serviceHint = computed(() => {
  const s = services.value.find(x => x.id === form.serviceId)
  if (!s) return '客人点“预约同款”时只预选化妆师，项目自己挑。'
  if (s.hidden) return '这个项目下架了，客人点“预约同款”时不会预选它，等重新上架就好。'
  return '客人点“预约同款”时会预选这个项目和化妆师。'
})

// ---------- 加载 ----------

function fill(f: Form) {
  saved.value = { ...f }
  Object.assign(form, f)
}

async function load() {
  status.value = 'loading'
  try {
    const [works, a, s] = await Promise.all([
      id.value ? api.listOwnerWorks() : Promise.resolve([]), api.listArtists(), api.listOwnerServices(),
    ])
    artists.value = a
    services.value = s
    if (id.value) {
      const w = works.find(x => x.id === id.value)
      if (!w) {
        errorMsg.value = '这件作品已经被删掉了'
        status.value = 'error'
        return
      }
      fill(toForm(w))
      uni.setNavigationBarTitle({ title: w.title })
    } else {
      // 只有一位化妆师时直接选上
      fill({ ...EMPTY, artistId: a.length === 1 ? a[0].id : '' })
    }
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad((query) => {
  id.value = query?.id ?? ''
  if (!id.value) uni.setNavigationBarTitle({ title: '添加作品' })
  load()
})

// ---------- 保存 ----------

const saving = ref(false)

async function save() {
  if (!dirty.value || saving.value) return
  const input = toInput(form)
  const msg = validateWork(input)
  if (msg) {
    uni.showToast({ title: msg, icon: 'none', duration: 2500 })
    return
  }
  saving.value = true
  try {
    const w = id.value ? await api.updateWork(id.value, input) : await api.createWork(input)
    fill(toForm(w))
    uni.showToast({ title: id.value ? '存好了' : '添加好了', icon: 'none', duration: 1200 })
    setTimeout(() => uni.navigateBack(), 1200)
  } catch (e) {
    // 化妆师或项目刚被删掉：提示里说了换一个
    uni.showToast({ title: errorText(e), icon: 'none', duration: 3000 })
  } finally {
    saving.value = false
  }
}

// ---------- 删除 ----------

const removing = ref(false)

function remove() {
  if (removing.value) return
  uni.showModal({
    title: `删除${saved.value.title}？`,
    content: '删掉后客人在“作品”里就看不到了。',
    confirmText: '删除',
    cancelText: '先不删',
    success: async (r) => {
      if (!r.confirm) return
      removing.value = true
      try {
        await api.deleteWork(id.value)
        uni.showToast({ title: '删好了', icon: 'none', duration: 1200 })
        setTimeout(() => uni.navigateBack(), 1200)
      } catch (e) {
        uni.showToast({ title: errorText(e), icon: 'none', duration: 3000 })
      } finally {
        removing.value = false
      }
    },
  })
}
</script>

<style lang="scss">
.pad {
  padding: 0 $page-x;
}

.preview {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding-top: 32rpx;

  // 作品页一格的宽度：(750 - 左右 32 - 中间 gap) / 2，取整
  &__cell {
    width: 320rpx;
  }

  &__title {
    margin-top: 16rpx;
    font-size: $fs-small;
    font-weight: 600;
    color: $ink;
  }

  &__artist {
    font-size: $fs-caption;
    color: $mute;
  }

  &__note {
    margin-top: 16rpx;
    font-size: $fs-caption;
    color: $disabled;
  }
}

.h {
  margin: 44rpx $page-x 20rpx;
  font-size: 30rpx;
  font-weight: 600;

  &__opt {
    margin-left: 16rpx;
    font-size: $fs-caption;
    font-weight: normal;
    color: $mute;
  }
}

.hint {
  margin: 16rpx $page-x 0;
  font-size: $fs-caption;
  line-height: 1.6;
  color: $mute;
}

.field {
  box-sizing: border-box;
  width: 100%;
  height: 92rpx;
  padding: 0 28rpx;
  border-radius: $r-inner;
  background: $card;
  font-size: $fs-body;
  color: $ink;

  &__ph {
    color: $disabled;
  }
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
  padding: 0 $page-x;
}

.pics {
  display: flex;
  justify-content: space-between;
  padding: 0 $page-x;

  &__item {
    padding: 6rpx;
    border-radius: $r-arch;

    &--on {
      box-shadow: 0 0 0 3rpx $rose;
    }
  }

  &__img {
    width: 88rpx;
    height: 110rpx;
  }
}

.policy {
  margin-top: 44rpx;
  padding: 24rpx 28rpx;
  border-radius: $r-inner;
  background: $sage-bg;
  font-size: $fs-caption + 2rpx;
  line-height: 1.7;
  color: $sage-ink;
}

.remove {
  display: flex;
  justify-content: center;
  margin: 64rpx 0 48rpx;
}

.gap {
  height: 48rpx;
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
