<template>
  <div class="page-container animate-fade-in">
    <PageHeader title="偏好设置" description="管理你的个人功能配置" />

    <div v-if="errorMsg" class="surface rounded-2xl px-5 py-3 text-sm text-destructive">
      {{ errorMsg }}
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="i in 4" :key="i" class="h-20 rounded-xl bg-muted animate-pulse"></div>
    </div>

    <ConfigGroupList
      v-else
      ref="listRef"
      :items="items"
      @save="handleSave"
    />

    <!-- 支持作者（仅管理员可见，赞赏完全自愿） -->
    <div v-if="authStore.isAdmin" class="surface rounded-2xl p-4 sm:p-6">
      <div class="flex flex-wrap items-center justify-between gap-4">
        <div class="flex items-start gap-3">
          <div class="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-amber-400/15 text-amber-500">
            <svg class="h-5 w-5" fill="currentColor" viewBox="0 0 24 24"><path d="M12 21.35l-1.45-1.32C5.4 15.36 2 12.28 2 8.5 2 5.42 4.42 3 7.5 3c1.74 0 3.41.81 4.5 2.09C13.09 3.81 14.76 3 16.5 3 19.58 3 22 5.42 22 8.5c0 3.78-3.4 6.86-8.55 11.54L12 21.35z"/></svg>
          </div>
          <div>
            <h3 class="text-sm font-bold text-foreground">支持作者</h3>
            <p class="mt-1 text-sm leading-relaxed text-muted-foreground">
              这个应用免费、无广告，数据完全保存在你自己的设备上。
              如果它帮到了你，欢迎请作者喝杯咖啡——金额随意，1 元也是心意；
              <strong class="text-foreground">不赞赏不影响任何功能</strong>。
            </p>
            <p v-if="supportStore.donateSupported" class="mt-2 text-xs text-emerald-500">
              ❤ 感谢你的支持，这也太暖了{{ supportStore.supportedVersion ? `（版本 ${supportStore.supportedVersion}）` : '' }}
            </p>
          </div>
        </div>
        <button class="btn-brand flex-1 sm:flex-none" @click="supportStore.open()">
          {{ supportStore.donateSupported ? '再次赞赏' : '去赞赏' }}
        </button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import PageHeader from '../components/PageHeader.vue'
import ConfigGroupList from '../components/ConfigGroupList.vue'
import { getUserConfigMeta, updateConfig } from '../api/config'
import type { ConfigMeta } from '../api/config'
import { useThemeStore } from '../stores/theme'
import { useAuthStore } from '../stores/auth'
import { useSupportStore } from '../stores/support'

const authStore = useAuthStore()
const supportStore = useSupportStore()
const items = ref<ConfigMeta[]>([])
const loading = ref(false)
const errorMsg = ref('')
const listRef = ref<InstanceType<typeof ConfigGroupList> | null>(null)

onMounted(loadMeta)

async function loadMeta() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getUserConfigMeta()
    if (res.data?.code === 0) {
      items.value = Array.isArray(res.data.data) ? res.data.data : []
    } else {
      errorMsg.value = res.data?.message || '加载配置失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}

async function handleSave(key: string, value: string) {
  errorMsg.value = ''
  try {
    const res = await updateConfig(key, value)
    if (res.data?.code === 0) {
      const idx = items.value.findIndex((it) => it.key === key)
      if (idx >= 0) {
        items.value[idx] = { ...items.value[idx], value }
      }
      listRef.value?.markSaved(key)
      if (key === 'theme_mode') {
        const themeStore = useThemeStore()
        if (themeStore.mode !== value) {
          themeStore.setMode(value as 'light' | 'dark' | 'system')
        }
      }
    } else {
      errorMsg.value = res.data?.message || '保存失败'
      listRef.value?.markFailed(key)
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
    listRef.value?.markFailed(key)
  }
}
</script>
