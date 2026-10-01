<template>
  <PageLayout>
    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="status === 'ok'">
      <view class="h">图片<text class="h__opt">按点的顺序排，第一张是封面</text></view>
      <view class="pics">
        <view v-for="src in PICS" :key="src" class="pics__item" @tap="togglePic(src)">
          <ArchImage class="pics__img" :class="{ 'pics__img--on': picOrder(src) }" shape="rect" :src="src" />
          <text v-if="picOrder(src)" class="pics__num">{{ picOrder(src) === 1 ? '封面' : picOrder(src) }}</text>
        </view>
      </view>
      <view class="hint">演示版先从这几个里选，以后可以上传照片。</view>

      <view class="h">项目名</view>
      <view class="pad">
        <input v-model="form.name" class="field" placeholder="比如 韩式上镜妆（含发型）" placeholder-class="field__ph" :maxlength="LIMITS.serviceName" />
      </view>

      <view class="h">一句话介绍</view>
      <view class="pad">
        <input v-model="form.summary" class="field" placeholder="比如 适合拍照、证件照" placeholder-class="field__ph" :maxlength="LIMITS.summary" />
      </view>

      <view class="h">风格</view>
      <view class="chips">
        <Chip v-for="c in CATEGORIES" :key="c" :label="CATEGORY_LABEL[c]" :selected="form.category === c" @click="form.category = c" />
      </view>

      <view class="h">时长</view>
      <view class="stepper">
        <view class="stepper__btn" :class="{ 'stepper__btn--off': form.durationMin <= LIMITS.minDuration }" @tap="stepDuration(-1)">−</view>
        <view class="stepper__value">约 {{ form.durationMin }} 分钟</view>
        <view class="stepper__btn" :class="{ 'stepper__btn--off': form.durationMin >= LIMITS.maxDuration }" @tap="stepDuration(1)">+</view>
      </view>

      <view class="h">价格</view>
      <view class="money">
        <view class="money__box">
          <text class="money__label">价格</text>
          <input v-model="form.price" class="money__input" type="digit" placeholder="0" placeholder-class="field__ph" />
          <text class="money__unit">元</text>
        </view>
        <view class="money__box">
          <text class="money__label">定金</text>
          <input v-model="form.deposit" class="money__input" type="digit" placeholder="0" placeholder-class="field__ph" />
          <text class="money__unit">元</text>
        </view>
      </view>
      <view class="chips chips--top">
        <Chip label="价格后面写“起”" :selected="form.priceFrom" @click="form.priceFrom = !form.priceFrom" />
      </view>
      <view class="pad">
        <view class="policy">客人下单时付定金，到店付尾款。{{ id ? '改价只影响之后的新预约，已经下的单按原价。' : '' }}</view>
      </view>

      <view class="h">包含内容<text class="h__opt">一行写一条</text></view>
      <view class="pad">
        <textarea
          v-model="form.includes"
          class="note"
          placeholder="比如：妆前护肤和底妆（换行写下一条）"
          placeholder-class="field__ph"
          auto-height
          disable-default-padding
          :maxlength="400"
          :cursor-spacing="120"
        />
      </view>

      <view class="h">标签<text class="h__opt">一行写一个，最多 {{ LIMITS.tags }} 个</text></view>
      <view class="pad">
        <textarea
          v-model="form.tags"
          class="note"
          placeholder="比如：拍照不假面（换行写下一个）"
          placeholder-class="field__ph"
          auto-height
          disable-default-padding
          :maxlength="80"
          :cursor-spacing="120"
        />
      </view>

      <template v-if="id">
        <view class="h">接不接预约</view>
        <view class="chips">
          <Chip label="在接预约" :selected="!form.hidden" @click="form.hidden = false" />
          <Chip label="暂停（下架）" :selected="form.hidden" @click="form.hidden = true" />
        </view>
        <view class="hint">下架后客人看不到这个项目，已经约了的照常进行。</view>

        <view class="remove">
          <AppButton variant="ghost" size="sm" :loading="removing" loading-text="正在删除…" @click="remove">删除这个项目</AppButton>
        </view>
      </template>
      <view v-else class="gap" />
    </template>

    <template #footer>
      <BottomBar v-if="status === 'ok'" :title="priceTitle" :desc="depositDesc">
        <AppButton :disabled="!dirty" :loading="saving" loading-text="正在保存…" @click="save">
          {{ id ? '保存修改' : '添加项目' }}
        </AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import {
  api, errorText, cleanService, validateService, LIMITS, CATEGORY_LABEL,
  type ID, type Service, type ServiceInput, type StyleCategory,
} from '@/api'
import { centsToYuanInput, formatPrice, parseYuan } from '@/utils/money'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import Chip from '@/components/Chip.vue'
import PageLayout from '@/components/PageLayout.vue'

const PICS = ['g1', 'g2', 'g3', 'g4', 'g5', 'g6'].map(g => `placeholder:${g}`)
const CATEGORIES = Object.keys(CATEGORY_LABEL) as StyleCategory[]

const id = ref<ID>('')
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

/** 金额输入框用元的字符串，包含内容和标签用多行文本，提交时再转 */
interface Form {
  name: string; summary: string; category?: StyleCategory; durationMin: number
  price: string; deposit: string; priceFrom: boolean
  includes: string; tags: string; images: string[]; hidden: boolean
}
const EMPTY: Form = {
  name: '', summary: '', category: undefined, durationMin: 90,
  price: '', deposit: '', priceFrom: false, includes: '', tags: '', images: [], hidden: false,
}
const form = reactive<Form>({ ...EMPTY, images: [] })
const saved = ref<Form>({ ...EMPTY })

const splitLines = (s: string) => s.split('\n')

function toInput(f: Form): ServiceInput {
  return cleanService({
    name: f.name, summary: f.summary, category: f.category as StyleCategory, durationMin: f.durationMin,
    price: parseYuan(f.price), deposit: parseYuan(f.deposit), priceFrom: f.priceFrom,
    includes: splitLines(f.includes), tags: splitLines(f.tags),
    cover: f.images[0] ?? '', images: f.images, hidden: f.hidden,
  })
}

function toForm(s: Service): Form {
  return {
    name: s.name, summary: s.summary, category: s.category, durationMin: s.durationMin,
    price: centsToYuanInput(s.price), deposit: centsToYuanInput(s.deposit), priceFrom: !!s.priceFrom,
    includes: s.includes.join('\n'), tags: s.tags.join('\n'), images: [...s.images], hidden: !!s.hidden,
  }
}

const dirty = computed(() => JSON.stringify(toInput(form)) !== JSON.stringify(toInput(saved.value)))

// ---------- 图片：点一下加到最后，再点一下去掉 ----------

/** 第几张（从 1 开始），没选返回 0 */
const picOrder = (src: string) => form.images.indexOf(src) + 1

function togglePic(src: string) {
  const i = form.images.indexOf(src)
  if (i >= 0) form.images.splice(i, 1)
  else if (form.images.length < LIMITS.images) form.images.push(src)
}

// ---------- 时长 ----------

function stepDuration(dir: 1 | -1) {
  const next = form.durationMin + dir * LIMITS.durationStep
  if (next >= LIMITS.minDuration && next <= LIMITS.maxDuration) form.durationMin = next
}

// ---------- 底部摘要：客人看到的价格 ----------

const priceTitle = computed(() => {
  const p = parseYuan(form.price)
  return Number.isNaN(p) ? '价格还没填' : formatPrice(p, { from: form.priceFrom })
})
const depositDesc = computed(() => {
  const d = parseYuan(form.deposit)
  return Number.isNaN(d) ? '定金还没填' : `定金 ${formatPrice(d)}，到店付尾款`
})

// ---------- 加载 ----------

function fill(f: Form) {
  saved.value = { ...f, images: [...f.images] }
  Object.assign(form, { ...f, images: [...f.images] })
}

async function load() {
  status.value = 'loading'
  try {
    if (id.value) {
      const s = (await api.listOwnerServices()).find(x => x.id === id.value)
      if (!s) {
        errorMsg.value = '这个项目已经被删掉了'
        status.value = 'error'
        return
      }
      fill(toForm(s))
      uni.setNavigationBarTitle({ title: s.name })
    } else {
      fill({ ...EMPTY, images: [] })
    }
    status.value = 'ok'
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad((query) => {
  id.value = query?.id ?? ''
  if (!id.value) uni.setNavigationBarTitle({ title: '添加项目' })
  load()
})

// ---------- 保存 ----------

const saving = ref(false)

/**
 * 输入框的元认不出时 parseYuan 给 NaN，validateService 会说“价格在 1–… 元之间”“定金至少 1 元”，
 * 这种情况换成更直接的说法
 */
function checkForm(f: Form, input: ServiceInput) {
  const msg = validateService(input)
  if (msg?.startsWith('价格') && Number.isNaN(parseYuan(f.price))) return '价格填数字，最多两位小数'
  if (msg?.startsWith('定金') && Number.isNaN(parseYuan(f.deposit))) return '定金填数字，最多两位小数'
  return msg
}

async function save() {
  if (!dirty.value || saving.value) return
  const input = toInput(form)
  const msg = checkForm(form, input)
  if (msg) {
    uni.showToast({ title: msg, icon: 'none', duration: 2500 })
    return
  }
  saving.value = true
  try {
    const s = id.value ? await api.updateService(id.value, input) : await api.createService(input)
    fill(toForm(s))
    uni.showToast({ title: id.value ? '存好了' : '添加好了', icon: 'none', duration: 1200 })
    setTimeout(() => uni.navigateBack(), 1200)
  } catch (e) {
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
    title: `删除${saved.value.name}？`,
    content: '删掉后客人就看不到也约不到了，关联它的作品不再显示“预约同款”的项目。只是暂时不接的话，用“暂停（下架）”就好。',
    confirmText: '删除',
    cancelText: '先不删',
    success: async (r) => {
      if (!r.confirm) return
      removing.value = true
      try {
        await api.deleteService(id.value)
        uni.showToast({ title: '删好了', icon: 'none', duration: 1200 })
        setTimeout(() => uni.navigateBack(), 1200)
      } catch (e) {
        // 还有没结束的预约、是最后一个在接预约的项目：后端的提示说清楚了怎么办
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

.note {
  box-sizing: border-box;
  width: 100%;
  min-height: 144rpx;
  padding: 24rpx 28rpx;
  border-radius: $r-inner;
  background: $card;
  font-size: $fs-small;
  line-height: 1.7;
  color: $ink;
}

.chips {
  display: flex;
  flex-wrap: wrap;
  gap: 16rpx;
  padding: 0 $page-x;

  &--top {
    margin-top: 20rpx;
  }
}

.pics {
  display: flex;
  justify-content: space-between;
  padding: 0 $page-x;

  &__item {
    position: relative;
  }

  &__img {
    width: 96rpx;
    height: 96rpx;
    border-radius: $r-small;

    &--on {
      box-shadow: 0 0 0 4rpx $rose;
    }
  }

  &__num {
    position: absolute;
    left: 50%;
    bottom: -14rpx;
    transform: translateX(-50%);
    padding: 0 12rpx;
    border-radius: $r-pill;
    background: $mocha;
    font-size: 18rpx;
    line-height: 30rpx;
    color: $card;
    white-space: nowrap;
  }
}

.stepper {
  display: flex;
  align-items: center;
  margin: 0 $page-x;
  border-radius: $r-inner;
  background: $card;

  &__btn {
    width: 112rpx;
    height: 92rpx;
    line-height: 92rpx;
    text-align: center;
    font-size: 40rpx;
    color: $mocha;

    &--off {
      color: $disabled;
    }
  }

  &__value {
    flex: 1;
    text-align: center;
    font-size: $fs-body;
    color: $ink;
  }
}

.money {
  display: flex;
  gap: 20rpx;
  padding: 0 $page-x;

  &__box {
    flex: 1;
    display: flex;
    align-items: center;
    height: 92rpx;
    padding: 0 24rpx;
    border-radius: $r-inner;
    background: $card;
  }

  &__label {
    flex: none;
    font-size: $fs-small;
    color: $mute;
  }

  &__input {
    flex: 1;
    min-width: 0;
    padding: 0 12rpx;
    text-align: right;
    font-size: $fs-body;
    color: $ink;
  }

  &__unit {
    flex: none;
    font-size: $fs-small;
    color: $mute;
  }
}

.policy {
  margin-top: 24rpx;
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
