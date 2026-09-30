<template>
  <PageLayout>
    <template #header>
      <view class="psub">填一次就好，以后每次预约，化妆师都会提前看到，不用每次再说一遍。</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="status === 'ok'">
      <view class="h">肤质</view>
      <view class="chips">
        <Chip
          v-for="s in SKINS"
          :key="s"
          :label="SKIN_LABEL[s]"
          :selected="form.skinType === s"
          @click="form.skinType = form.skinType === s ? undefined : s"
        />
      </view>

      <view class="h">肤色</view>
      <view class="chips">
        <Chip
          v-for="t in TONES"
          :key="t"
          :label="TONE_LABEL[t]"
          :selected="form.tone === t"
          @click="form.tone = form.tone === t ? undefined : t"
        />
      </view>

      <view class="h">过敏或不能用的</view>
      <view class="pad">
        <textarea
          v-model="form.allergies"
          class="note"
          placeholder="比如对酒精过敏，或者用某款粉底会闷痘"
          placeholder-class="note__ph"
          :maxlength="200"
          auto-height
          disable-default-padding
          :cursor-spacing="120"
        />
      </view>

      <view class="h">还想让化妆师知道的</view>
      <view class="pad">
        <textarea
          v-model="form.note"
          class="note"
          placeholder="比如单眼皮、戴隐形眼镜，或者喜欢淡一点的妆"
          placeholder-class="note__ph"
          :maxlength="200"
          auto-height
          disable-default-padding
          :cursor-spacing="120"
        />
        <view class="policy">档案只给为你化妆的化妆师看，不会用在别的地方。随时可以改，也可以清空。</view>
      </view>
    </template>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :title="savedText" desc="预约时化妆师会提前看到">
        <AppButton :disabled="!dirty" :loading="saving" loading-text="正在保存…" @click="save">保存档案</AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  api, errorText, SKIN_LABEL, TONE_LABEL,
  type SkinProfile, type SkinTone, type SkinType, type UpdateSkinProfileReq,
} from '@/api'
import { formatMonthDay, toDateStr } from '@/utils/date'
import AppButton from '@/components/AppButton.vue'
import BottomBar from '@/components/BottomBar.vue'
import Chip from '@/components/Chip.vue'
import PageLayout from '@/components/PageLayout.vue'

const SKINS = Object.keys(SKIN_LABEL) as SkinType[]
const TONES = Object.keys(TONE_LABEL) as SkinTone[]

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

/** 表单里文本框用空字符串，保存时空字符串就是清空 */
const form = reactive<{ skinType?: SkinType; tone?: SkinTone; allergies: string; note: string }>({
  skinType: undefined, tone: undefined, allergies: '', note: '',
})
const saved = ref<SkinProfile>({})

const snapshot = (p: UpdateSkinProfileReq) =>
  JSON.stringify([p.skinType ?? '', p.tone ?? '', p.allergies?.trim() ?? '', p.note?.trim() ?? ''])

const dirty = computed(() => snapshot(form) !== snapshot(saved.value))

const savedText = computed(() => {
  const at = saved.value.updatedAt
  return at ? `上次更新 ${formatMonthDay(toDateStr(new Date(at)))}` : '还没填写过'
})

function fill(p: SkinProfile) {
  saved.value = p
  form.skinType = p.skinType
  form.tone = p.tone
  form.allergies = p.allergies ?? ''
  form.note = p.note ?? ''
}

async function load() {
  status.value = 'loading'
  try {
    fill(await api.getSkinProfile())
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad(load)

// ---------- 保存 ----------

const saving = ref(false)

async function save() {
  if (!dirty.value || saving.value) return
  saving.value = true
  try {
    fill(await api.updateSkinProfile({ ...form }))
    uni.showToast({ title: '存好了', icon: 'none', duration: 1200 })
    setTimeout(() => uni.navigateBack(), 1200)
  } catch (e) {
    uni.showToast({ title: errorText(e), icon: 'none', duration: 2500 })
  } finally {
    saving.value = false
  }
}
</script>

<style lang="scss">
.psub {
  padding: 8rpx $page-x 0;
  font-size: $fs-caption + 2rpx;
  line-height: 1.7;
  color: $mute;
}

.pad {
  padding: 0 $page-x;
}

.h {
  margin: 44rpx $page-x 24rpx;
  font-size: 30rpx;
  font-weight: 600;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
  padding: 0 $page-x;
}

.note {
  box-sizing: border-box;
  width: 100%;
  min-height: 144rpx;
  padding: 24rpx;
  border-radius: $r-inner;
  background: $card;
  font-size: $fs-small;
  line-height: 1.6;
  color: $ink;

  &__ph {
    color: $disabled;
  }
}

.policy {
  margin: 32rpx 0 40rpx;
  padding: 24rpx 28rpx;
  border-radius: $r-inner;
  background: $sage-bg;
  font-size: $fs-caption + 2rpx;
  line-height: 1.7;
  color: $sage-ink;
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
