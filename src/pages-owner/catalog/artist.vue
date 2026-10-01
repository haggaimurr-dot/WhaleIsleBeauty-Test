<template>
  <PageLayout>
    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="status === 'ok'">
      <view class="h">头像</view>
      <view class="avatars">
        <view
          v-for="src in AVATARS"
          :key="src"
          class="avatars__item"
          :class="{ 'avatars__item--on': form.avatar === src }"
          @tap="form.avatar = src"
        >
          <ArchImage class="avatars__img" :src="src" />
        </view>
      </view>
      <view class="hint">演示版先从这几个里选，以后可以上传照片。</view>

      <view class="h">名字</view>
      <view class="pad">
        <input v-model="form.name" class="field" placeholder="客人看到的称呼，比如 小鲸" placeholder-class="field__ph" :maxlength="LIMITS.artistName" />
      </view>

      <view class="h">头衔<text class="h__opt">可以不填</text></view>
      <view class="pad">
        <input v-model="form.title" class="field" placeholder="比如 主理人" placeholder-class="field__ph" :maxlength="LIMITS.artistTitle" />
      </view>

      <view class="h">擅长<text class="h__opt">可以不填</text></view>
      <view class="pad">
        <input v-model="form.specialty" class="field" placeholder="比如 擅长新娘" placeholder-class="field__ph" :maxlength="LIMITS.specialty" />
      </view>

      <view class="h">从业年限</view>
      <view class="years">
        <input v-model="form.years" class="field years__input" type="number" placeholder="0" placeholder-class="field__ph" :maxlength="2" />
        <text class="years__unit">年</text>
      </view>

      <view class="preview">
        <view class="preview__label">客人预约时看到</view>
        <view class="preview__name">{{ form.name.trim() || '名字' }}</view>
        <view class="preview__desc">{{ previewDesc }}</view>
      </view>

      <view v-if="id" class="remove">
        <AppButton variant="ghost" size="sm" :loading="removing" loading-text="正在删除…" @click="remove">删除这位化妆师</AppButton>
      </view>
    </template>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :desc="id ? '改了马上生效，已经下的单不受影响' : '添加后客人就能约 TA'">
        <AppButton :disabled="!dirty" :loading="saving" loading-text="正在保存…" @click="save">
          {{ id ? '保存修改' : '添加化妆师' }}
        </AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { api, errorText, cleanArtist, validateArtist, LIMITS, type Artist, type ArtistInput, type ID } from '@/api'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'

const AVATARS = ['g1', 'g2', 'g3', 'g4', 'g5', 'g6'].map(g => `placeholder:${g}`)

const id = ref<ID>('')
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

/** 年限输入框给的是字符串，提交时再转数字 */
interface Form { name: string; title: string; specialty: string; years: string; avatar: string }
const EMPTY: Form = { name: '', title: '', specialty: '', years: '', avatar: AVATARS[0] }
const form = reactive<Form>({ ...EMPTY })
const saved = ref<Form>({ ...EMPTY })

function toInput(f: Form): ArtistInput {
  return cleanArtist({
    name: f.name, title: f.title, specialty: f.specialty, avatar: f.avatar,
    years: f.years.trim() === '' ? NaN : Number(f.years),
  })
}

function toForm(a: Artist): Form {
  return { name: a.name, title: a.title ?? '', specialty: a.specialty ?? '', years: String(a.years), avatar: a.avatar }
}

const dirty = computed(() => JSON.stringify(toInput(form)) !== JSON.stringify(toInput(saved.value)))

const previewDesc = computed(() => {
  const years = form.years.trim() === '' ? '' : `${Number(form.years)} 年`
  return [form.title.trim(), years, form.specialty.trim()].filter(Boolean).join(' ') || '头衔、年限、擅长'
})

function fill(f: Form) {
  saved.value = { ...f }
  Object.assign(form, f)
}

async function load() {
  status.value = 'loading'
  try {
    if (id.value) {
      const a = (await api.listArtists()).find(x => x.id === id.value)
      if (!a) {
        errorMsg.value = '这位化妆师已经被删掉了'
        status.value = 'error'
        return
      }
      fill(toForm(a))
      uni.setNavigationBarTitle({ title: a.name })
    } else {
      // 新建：保存按钮在填了名字之后才亮，所以和空表单比较就行
      fill({ ...EMPTY })
    }
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad((query) => {
  id.value = query?.id ?? ''
  if (!id.value) uni.setNavigationBarTitle({ title: '添加化妆师' })
  load()
})

// ---------- 保存 ----------

const saving = ref(false)

async function save() {
  if (!dirty.value || saving.value) return
  const input = toInput(form)
  const msg = validateArtist(input)
  if (msg) {
    uni.showToast({ title: msg, icon: 'none', duration: 2500 })
    return
  }
  saving.value = true
  try {
    const a = id.value ? await api.updateArtist(id.value, input) : await api.createArtist(input)
    fill(toForm(a))
    uni.showToast({ title: id.value ? '存好了' : '添加好了', icon: 'none', duration: 1200 })
    setTimeout(() => uni.navigateBack(), 1200)
  } catch (e) {
    uni.showToast({ title: errorText(e), icon: 'none', duration: 2500 })
  } finally {
    saving.value = false
  }
}

// ---------- 删除 ----------

const removing = ref(false)

function remove() {
  if (removing.value) return
  uni.showModal({
    title: `删除${saved.value.name}？`,
    content: '删掉后客人就约不到 TA 了，排班表里也不再显示。已经结束的预约记录还在。',
    confirmText: '删除',
    cancelText: '先不删',
    success: async (r) => {
      if (!r.confirm) return
      removing.value = true
      try {
        await api.deleteArtist(id.value)
        uni.showToast({ title: '删好了', icon: 'none', duration: 1200 })
        setTimeout(() => uni.navigateBack(), 1200)
      } catch (e) {
        // 还有预约、名下有作品、只剩一位：后端的提示已经说清楚了怎么办
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

.avatars {
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
    height: 104rpx;
  }
}

.years {
  display: flex;
  align-items: center;
  gap: 20rpx;
  padding: 0 $page-x;

  &__input {
    width: 180rpx;
    text-align: center;
  }

  &__unit {
    font-size: $fs-body;
    color: $mute;
  }
}

.preview {
  margin: 48rpx $page-x 0;
  padding: 28rpx 32rpx;
  border-radius: $r-inner;
  box-shadow: inset 0 0 0 2rpx $hair;

  &__label {
    margin-bottom: 12rpx;
    font-size: $fs-caption;
    color: $mute;
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
}

.remove {
  display: flex;
  justify-content: center;
  margin: 64rpx 0 48rpx;
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
