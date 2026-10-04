<template>
  <PageLayout>
    <template #header>
      <view v-if="service" class="chosen" hover-class="chosen--hover" @tap="changeService">
        <ArchImage class="chosen__thumb" shape="rect" :src="service.cover" />
        <view class="chosen__text">
          <view class="chosen__name">{{ service.name }}</view>
          约 {{ service.durationMin }} 分钟　{{ formatPrice(service.price, { from: service.priceFrom }) }}
        </view>
        <text class="chosen__change">换一个</text>
      </view>
      <view v-if="service && showAgainHint" class="hint hint--head">照你上次约的选好了项目和化妆师，挑个时间就行</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="service">
      <SlotPicker
        ref="picker"
        v-model:artist-id="artistId"
        v-model:date="date"
        v-model:time="time"
        :service-id="service.id"
        :artists="artists"
        :dates="dates"
      />

      <view class="h">这次是为了</view>
      <view class="chips">
        <Chip
          v-for="o in OCCASIONS"
          :key="o"
          :label="OCCASION_LABEL[o]"
          :selected="occasion === o"
          @click="occasion = occasion === o ? undefined : o"
        />
      </view>

      <view class="h">你的肤质</view>
      <view v-if="skinType && skinType === profileSkin" class="hint">按你的肤质档案选好了，这次不一样可以改</view>
      <view class="chips">
        <Chip
          v-for="s in SKINS"
          :key="s"
          :label="SKIN_LABEL[s]"
          :selected="skinType === s"
          @click="skinType = skinType === s ? undefined : s"
        />
      </view>

      <view class="h">还想告诉化妆师的</view>
      <view class="pad">
        <textarea
          v-model="note"
          class="note"
          placeholder="比如过敏的成分、喜欢的风格，或者带一张参考图来"
          placeholder-class="note__ph"
          :maxlength="200"
          auto-height
          disable-default-padding
          :cursor-spacing="120"
        />
        <view class="policy">预约时间前 24 小时可免费改期或取消，定金原路退回。你的照片只在你同意后才会用作作品展示。</view>
      </view>
    </template>

    <template #footer>
      <BottomBar v-if="service" :title="summary" :desc="`定金 ${formatPrice(service.deposit)}，到店付尾款`">
        <AppButton :disabled="!time" :loading="paying" loading-text="正在预约…" @click="submit">
          付定金并预约
        </AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { onLoad, onShow } from '@dcloudio/uni-app'
import {
  api, payDeposit, errorText, ApiError, OCCASION_LABEL, SKIN_LABEL,
  type Artist, type CreateBookingResp, type DateStr, type ID, type Occasion,
  type Service, type SkinType, type TimeStr,
} from '@/api'
import { formatDateCN } from '@/utils/date'
import { formatPrice } from '@/utils/money'
import { requestSubscribe } from '@/utils/subscribe'
import { takeTabParams, type TabParams } from '@/utils/tab'
import AppButton from '@/components/AppButton.vue'
import ArchImage from '@/components/ArchImage.vue'
import BottomBar from '@/components/BottomBar.vue'
import Chip from '@/components/Chip.vue'
import PageLayout from '@/components/PageLayout.vue'
import SlotPicker from '@/components/SlotPicker.vue'

const OCCASIONS = Object.keys(OCCASION_LABEL) as Occasion[]
const SKINS = Object.keys(SKIN_LABEL) as SkinType[]

// ---------- 基础数据 ----------

const services = ref<Service[]>([])
const artists = ref<Artist[]>([])
const dates = ref<DateStr[]>([])
const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

// ---------- 表单 ----------

const serviceId = ref<ID>()
const artistId = ref<ID>()
const date = ref<DateStr>()
const time = ref<TimeStr>()
const occasion = ref<Occasion>()
const skinType = ref<SkinType>()
/** 肤质档案里的肤质，用来预选；客人手动改过就不再跟着档案变 */
const profileSkin = ref<SkinType>()

/** 档案拿不到不影响预约，当作没填 */
async function syncProfileSkin() {
  const p = await api.getSkinProfile().catch(() => ({ skinType: undefined }))
  if (skinType.value === profileSkin.value) skinType.value = p.skinType
  profileSkin.value = p.skinType
}
const note = ref('')

const service = computed(() => services.value.find(s => s.id === serviceId.value))
const picker = ref<InstanceType<typeof SlotPicker>>()

const summary = computed(() => {
  const artist = artists.value.find(a => a.id === artistId.value)
  return date.value && time.value && artist
    ? `${formatDateCN(date.value)} ${time.value}　${artist.name}`
    : '还没选时间'
})

/** 从作品页“预约同款”、详情页“预约这个妆”带过来的预选 */
let pendingParams: TabParams['booking']

/** “再约一次”预选的组合；客人换了项目或化妆师，提示就收起 */
const again = ref<{ serviceId: ID; artistId: ID }>()
const showAgainHint = computed(() =>
  !!again.value && again.value.serviceId === serviceId.value && again.value.artistId === artistId.value)

function applyParams() {
  const p = pendingParams
  if (!p || status.value !== 'ok') return
  pendingParams = undefined
  const serviceOk = !p.serviceId || services.value.some(s => s.id === p.serviceId)
  const artistOk = !p.artistId || artists.value.some(a => a.id === p.artistId)
  if (p.serviceId && serviceOk) serviceId.value = p.serviceId
  if (p.artistId && artistOk) artistId.value = p.artistId
  time.value = undefined
  again.value = p.again && p.serviceId && p.artistId && serviceOk && artistOk
    ? { serviceId: p.serviceId, artistId: p.artistId }
    : undefined
  // 下架的项目、不在了的化妆师选不上，说一声，免得客人以为还是原来那个
  const prefix = p.again ? '上次的' : '这个'
  if (!serviceOk && !artistOk) toast(`${prefix}项目和化妆师现在都约不了，换一个吧`)
  else if (!serviceOk) toast(`${prefix}项目暂时不接预约了，换一个吧`)
  else if (!artistOk) toast(p.again ? '上次的化妆师现在不接预约了，看看别的化妆师吧' : '这位化妆师现在不接预约了，看看别的化妆师吧')
}

async function load() {
  status.value = 'loading'
  try {
    const [s, a, d] = await Promise.all([api.listServices(), api.listArtists(), api.listBookableDates()])
    services.value = s
    artists.value = a
    dates.value = d
    if (!serviceId.value) serviceId.value = s[0]?.id
    if (!artistId.value) artistId.value = a[0]?.id
    if (!date.value) date.value = d[0]
    status.value = 'ok'
    applyParams()
  } catch (e) {
    errorMsg.value = errorText(e)
    status.value = 'error'
  }
}

onLoad(load)

onShow(() => {
  // 从“我的”改完档案再切回来，预选跟着变
  syncProfileSkin()
  // tab 页切回来时时段可能已经变了（别人约走了、自己刚约了），重新拉一次；首次进入由 SlotPicker 自己拉
  if (status.value === 'ok') picker.value?.refresh()
  const p = takeTabParams('booking')
  if (p) {
    pendingParams = p
    applyParams()
  }
})

function changeService() {
  uni.showActionSheet({
    itemList: services.value.map(s => s.name),
    success: ({ tapIndex }) => {
      const s = services.value[tapIndex]
      if (s && s.id !== serviceId.value) {
        serviceId.value = s.id
        time.value = undefined
      }
    },
  })
}

// ---------- 提交 ----------

const paying = ref(false)
/**
 * 已下单但没付完的预约。用户取消支付后，时段会被保留 15 分钟；
 * 选择没变时再点一次，用 resumePayment 重新拉起支付，不重复下单。已经超时就当新的一单重新下。
 */
let unpaid: { key: string; resp: CreateBookingResp } | undefined

const toast = (title: string) => uni.showToast({ title, icon: 'none', duration: 2500 })

async function submit() {
  const s = service.value
  if (!s || !artistId.value || !date.value || !time.value || paying.value) return
  const req = {
    serviceId: s.id, artistId: artistId.value, date: date.value, time: time.value,
    occasion: occasion.value, skinType: skinType.value, note: note.value.trim() || undefined,
  }
  const key = [req.serviceId, req.artistId, req.date, req.time].join('|')

  paying.value = true
  try {
    // 必须是点击后第一个异步调用；等订阅弹窗关掉再拉起支付，两个弹窗不叠在一起
    await requestSubscribe(['confirmed', 'reminder'])
    const resp = (unpaid?.key === key && await resume(unpaid.resp.booking.id)) || await api.createBooking(req)
    unpaid = { key, resp }
    const booking = await payDeposit(resp)
    unpaid = undefined
    resetForm()
    uni.navigateTo({ url: `/pages/booking/success?id=${booking.id}` })
  } catch (e) {
    if (e instanceof ApiError && e.code === 'SLOT_TAKEN') {
      time.value = undefined
      picker.value?.refresh()
    }
    toast(errorText(e))
  } finally {
    paying.value = false
  }
}

/** 重新拿支付参数；已超时或已处理返回 undefined */
async function resume(id: ID) {
  try {
    return await api.resumePayment(id)
  } catch (e) {
    if (e instanceof ApiError && e.code === 'INVALID_STATE') return undefined
    throw e
  }
}

function resetForm() {
  time.value = undefined
  again.value = undefined
  occasion.value = undefined
  skinType.value = profileSkin.value
  note.value = ''
  // 刚约的时段要显示为已约满
  picker.value?.refresh()
}
</script>

<style lang="scss">
.pad {
  padding: 0 $page-x;
}

.chosen {
  display: flex;
  align-items: center;
  gap: $gap;
  margin: 8rpx $page-x 8rpx;
  padding: 20rpx;
  border-radius: 32rpx;
  background: $card;
  font-size: $fs-small;
  color: $mute;

  &--hover {
    opacity: 0.85;
  }

  &__thumb {
    flex: none;
    width: 96rpx;
    height: 96rpx;
    border-radius: $r-small;
  }

  &__text {
    flex: 1;
    min-width: 0;
  }

  &__name {
    font-size: $fs-body;
    font-weight: 600;
    color: $ink;
  }

  &__change {
    flex: none;
    padding-right: 8rpx;
    font-size: $fs-caption + 2rpx;
  }
}

.h {
  margin: 44rpx $page-x 24rpx;
  font-size: 30rpx;
  font-weight: 600;
}

.hint {
  margin: -12rpx $page-x 20rpx;
  font-size: $fs-caption;
  color: $mute;

  &--head {
    margin: 8rpx $page-x 0;
  }
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
