<template>
  <PageLayout>
    <template #header>
      <view class="psub">客人在预约成功页、联系门店和到店导航里会看到这些。</view>
    </template>

    <view v-if="status === 'error'" class="panel">
      <view class="panel__text">{{ errorMsg }}</view>
      <AppButton variant="ghost" size="sm" @click="load">再试一次</AppButton>
    </view>

    <template v-else-if="status === 'ok'">
      <view class="h">店名</view>
      <view class="pad">
        <input v-model="form.name" class="field" placeholder="比如 鲸屿美妆" placeholder-class="field__ph" :maxlength="LIMITS.shopName" />
      </view>

      <view class="h">电话</view>
      <view class="pad">
        <input
          v-model="form.phone"
          class="field"
          placeholder="客人点“联系门店”会拨这个号码"
          placeholder-class="field__ph"
          :maxlength="LIMITS.phone"
        />
      </view>

      <view class="h">营业时间</view>
      <view class="hours">
        <picker mode="time" :value="hoursFrom" @change="pickFrom">
          <view class="field field--pick">{{ hoursFrom }}</view>
        </picker>
        <text class="hours__to">到</text>
        <picker mode="time" :value="hoursTo" @change="pickTo">
          <view class="field field--pick">{{ hoursTo }}</view>
        </picker>
      </view>

      <view class="h">地址</view>
      <view class="pad">
        <input
          v-model="form.address"
          class="field"
          placeholder="写到门牌号，客人照着能找到"
          placeholder-class="field__ph"
          :maxlength="LIMITS.address"
          :cursor-spacing="120"
        />
        <view class="loc">
          <view class="loc__text">
            <view class="loc__title">地图位置</view>
            <view class="loc__sub">{{ locationText }}</view>
          </view>
          <AppButton v-if="hasLocation" variant="ghost" size="sm" @click="previewLocation">看一下</AppButton>
          <AppButton variant="ghost" size="sm" @click="chooseLocation">{{ hasLocation ? '重新选' : '在地图上选' }}</AppButton>
        </view>
      </view>
    </template>

    <template #footer>
      <BottomBar v-if="status === 'ok'" desc="改了马上生效">
        <AppButton :disabled="!dirty" :loading="saving" loading-text="正在保存…" @click="save">保存门店信息</AppButton>
      </BottomBar>
    </template>
  </PageLayout>
</template>

<script setup lang="ts">
import { computed, reactive, ref } from 'vue'
import { onLoad } from '@dcloudio/uni-app'
import { api, errorText, cleanShop, validateShop, LIMITS, type Shop } from '@/api'
import AppButton from '@/components/AppButton.vue'
import BottomBar from '@/components/BottomBar.vue'
import PageLayout from '@/components/PageLayout.vue'

const status = ref<'loading' | 'ok' | 'error'>('loading')
const errorMsg = ref('')

const form = reactive<Shop>({ name: '', address: '', phone: '', openHours: '', latitude: 0, longitude: 0 })
const saved = ref<Shop>()

// ---------- 营业时间：两个时间选择器，存成 '09:00–21:00' ----------

const DEFAULT_HOURS = ['09:00', '21:00']
const hoursFrom = ref(DEFAULT_HOURS[0])
const hoursTo = ref(DEFAULT_HOURS[1])

/** 认不出的写法（比如以前手填的）就从默认的 9 点到 21 点开始选 */
function parseHours(s: string) {
  const m = /^(\d{2}:\d{2})\s*[–~-]\s*(\d{2}:\d{2})$/.exec(s.trim())
  return m ? [m[1], m[2]] : DEFAULT_HOURS
}

function pickFrom(e: { detail: { value: string } }) {
  hoursFrom.value = e.detail.value
  form.openHours = `${hoursFrom.value}–${hoursTo.value}`
}

function pickTo(e: { detail: { value: string } }) {
  hoursTo.value = e.detail.value
  form.openHours = `${hoursFrom.value}–${hoursTo.value}`
}

// ---------- 地图位置 ----------

/** 这次在地图上选到的地点名，只用于显示；坐标才是存下来的 */
const pickedName = ref('')
const hasLocation = computed(() => !(form.latitude === 0 && form.longitude === 0))
const locationText = computed(() => {
  if (pickedName.value) return `已选：${pickedName.value}`
  return hasLocation.value ? '已经选好，客人点“到店导航”会打开这里' : '还没选，客人没法导航过来'
})

function chooseLocation() {
  uni.chooseLocation({
    ...(hasLocation.value ? { latitude: form.latitude, longitude: form.longitude } : {}),
    success: (r) => {
      form.latitude = r.latitude
      form.longitude = r.longitude
      pickedName.value = r.name || r.address
      // 地址还空着就顺手填上，已经写了的不动
      if (!form.address.trim() && r.address) form.address = r.address
    },
    fail: (err) => {
      if (/cancel/.test(err.errMsg)) return
      uni.showToast({ title: '地图没打开，稍后再试一次', icon: 'none', duration: 2500 })
    },
  })
}

function previewLocation() {
  uni.openLocation({ latitude: form.latitude, longitude: form.longitude, name: form.name, address: form.address })
}

// ---------- 加载 ----------

const snapshot = (s: Shop) => JSON.stringify(cleanShop(s))
const dirty = computed(() => !!saved.value && snapshot(form) !== snapshot(saved.value))

function fill(s: Shop) {
  saved.value = s
  Object.assign(form, s)
  ;[hoursFrom.value, hoursTo.value] = parseHours(s.openHours)
  pickedName.value = ''
}

async function load() {
  status.value = 'loading'
  try {
    fill(await api.getShop())
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
  const shop = cleanShop(form)
  const msg = hoursTo.value <= hoursFrom.value ? '关门时间要比开门时间晚' : validateShop(shop)
  if (msg) {
    uni.showToast({ title: msg, icon: 'none', duration: 2500 })
    return
  }
  saving.value = true
  try {
    fill(await api.updateShop(shop))
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
  margin: 44rpx $page-x 20rpx;
  font-size: 30rpx;
  font-weight: 600;
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

  &--pick {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 220rpx;
    font-variant-numeric: tabular-nums;
  }
}

.hours {
  display: flex;
  align-items: center;
  gap: 24rpx;
  padding: 0 $page-x;

  &__to {
    font-size: $fs-small;
    color: $mute;
  }
}

.loc {
  display: flex;
  align-items: center;
  gap: 16rpx;
  margin: 20rpx 0 40rpx;
  padding: 24rpx 28rpx;
  border-radius: $r-inner;
  background: $card;

  &__text {
    flex: 1;
    min-width: 0;
  }

  &__title {
    font-size: $fs-small;
    color: $ink;
  }

  &__sub {
    margin-top: 4rpx;
    font-size: $fs-caption;
    line-height: 1.5;
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
