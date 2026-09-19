<template>
  <div class="page-container animate-fade-in">
    <!-- Profile hero：手机端收成一行式紧凑头部，桌面端保持大卡 -->
    <div class="relative overflow-hidden rounded-2xl bg-brand-gradient p-4 sm:p-8 shadow-glow">
      <div class="absolute -top-16 -right-10 w-64 h-64 rounded-full bg-white/10 blur-2xl"></div>
      <div class="absolute -bottom-20 -left-10 w-56 h-56 rounded-full bg-black/10 blur-2xl"></div>
      <div class="relative flex items-center gap-3 sm:gap-5">
        <div class="w-12 h-12 sm:w-20 sm:h-20 rounded-xl sm:rounded-2xl bg-white/15 backdrop-blur ring-1 ring-white/25 flex items-center justify-center text-white font-display font-extrabold text-xl sm:text-3xl shrink-0">
          {{ userInitial }}
        </div>
        <div class="min-w-0">
          <h1 class="text-lg sm:text-2xl font-display font-extrabold text-white truncate">{{ authStore.user?.username }}</h1>
          <div class="flex items-center gap-2 mt-1 sm:mt-2">
            <span class="badge bg-white/20 text-white backdrop-blur">
              <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M10 1l2.39 4.84L18 6.66l-4 3.9.94 5.5L10 13.5l-4.94 2.56.94-5.5-4-3.9 5.61-.82L10 1z"/></svg>
              {{ authStore.isAdmin ? '管理员' : '普通用户' }}
            </span>
            <span class="text-white/70 text-xs sm:text-sm truncate">@{{ authStore.user?.username }}</span>
          </div>
        </div>
      </div>
    </div>

    <div class="grid grid-cols-1 lg:grid-cols-2 gap-4 sm:gap-6">
      <!-- Change password -->
      <section class="surface rounded-2xl p-4 sm:p-7">
        <div class="flex items-center gap-2.5 sm:gap-3 mb-3 sm:mb-5">
          <div class="w-8 h-8 sm:w-10 sm:h-10 rounded-lg sm:rounded-xl bg-brand-50 dark:bg-brand-500/15 text-brand-600 dark:text-brand-300 flex items-center justify-center shrink-0">
            <svg class="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="9" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V8a4 4 0 118 0v3"/></svg>
          </div>
          <h2 class="text-base sm:text-lg font-bold text-foreground">修改密码</h2>
        </div>
        <form @submit.prevent="handleChangePassword" class="space-y-3 sm:space-y-4">
          <div>
            <label class="block text-[13px] sm:text-sm font-medium text-foreground mb-1">当前密码</label>
            <input v-model="oldPassword" type="password" required class="input-field" autofocus />
          </div>
          <div>
            <label class="block text-[13px] sm:text-sm font-medium text-foreground mb-1">新密码</label>
            <input v-model="newPassword" type="password" required class="input-field" />
          </div>
          <div>
            <label class="block text-[13px] sm:text-sm font-medium text-foreground mb-1">确认新密码</label>
            <input v-model="confirmPassword" type="password" required class="input-field" />
          </div>
          <p v-if="pwdMsg" :class="pwdSuccess ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'" class="text-sm">{{ pwdMsg }}</p>
          <button type="submit" :disabled="pwdLoading" class="btn-brand">{{ pwdLoading ? '修改中...' : '修改密码' }}</button>
        </form>
      </section>

      <!-- API Key -->
      <section class="surface rounded-2xl p-4 sm:p-7">
        <div class="flex items-center gap-2.5 sm:gap-3 mb-3 sm:mb-5">
          <div class="w-8 h-8 sm:w-10 sm:h-10 rounded-lg sm:rounded-xl bg-amber-50 dark:bg-amber-500/15 text-amber-600 dark:text-amber-300 flex items-center justify-center shrink-0">
            <svg class="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
          </div>
          <h2 class="text-base sm:text-lg font-bold text-foreground">API Key</h2>
        </div>
        <div class="space-y-3 sm:space-y-4">
          <p class="text-[13px] sm:text-sm text-muted-foreground">用于以编程方式访问接口，请妥善保管，泄露后请立即重新生成。</p>
          <div v-if="apiKey" class="flex items-center gap-2">
            <code class="flex-1 px-3 py-2 rounded-xl bg-muted border border-border text-[13px] sm:text-sm text-foreground font-mono break-all">{{ apiKey }}</code>
            <button @click="copyApiKey" class="btn-ghost !px-3 !py-2 shrink-0">复制</button>
          </div>
          <p v-if="apiKeyMsg" :class="apiKeySuccess ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'" class="text-sm">{{ apiKeyMsg }}</p>
          <button @click="handleRegenerateApiKey" :disabled="apiKeyLoading" class="btn-brand">{{ apiKeyLoading ? '生成中...' : '重新生成 API Key' }}</button>
        </div>
      </section>
    </div>

    <!-- Security questions -->
    <section class="surface rounded-2xl p-4 sm:p-7">
      <div class="flex items-center gap-2.5 sm:gap-3 mb-3 sm:mb-5">
        <div class="w-8 h-8 sm:w-10 sm:h-10 rounded-lg sm:rounded-xl bg-emerald-50 dark:bg-emerald-500/15 text-emerald-600 dark:text-emerald-300 flex items-center justify-center shrink-0">
          <svg class="w-4 h-4 sm:w-5 sm:h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
        </div>
        <h2 class="text-base sm:text-lg font-bold text-foreground">安全问题</h2>
      </div>

      <div v-if="hasQuestions" class="flex items-center gap-2 text-[13px] sm:text-sm text-emerald-600 dark:text-emerald-400 bg-emerald-50 dark:bg-emerald-500/10 border border-emerald-100 dark:border-emerald-500/20 rounded-xl px-3 py-2 sm:px-4 sm:py-3 mb-3 sm:mb-4">
        <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
        安全问题已设置，可用于找回密码
      </div>

      <p class="text-[13px] sm:text-sm text-muted-foreground mb-3 sm:mb-4">{{ hasQuestions ? '修改安全问题，忘记密码时用于身份验证。' : '设置三组安全问题，忘记密码时用于身份验证。' }}</p>
      <div class="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-3 sm:gap-4">
        <div v-for="n in 3" :key="n" class="space-y-2.5 sm:space-y-3">
          <div>
            <label class="block text-[13px] sm:text-sm font-medium text-foreground mb-1">问题 {{ n }}</label>
            <input v-model="qa[n - 1].q" type="text" class="input-field" :placeholder="`安全问题 ${n}`" />
          </div>
          <div>
            <label class="block text-[13px] sm:text-sm font-medium text-foreground mb-1">答案 {{ n }}</label>
            <input v-model="qa[n - 1].a" type="text" class="input-field" :placeholder="`答案 ${n}`" />
          </div>
        </div>
      </div>
      <p v-if="secMsg" :class="secSuccess ? 'text-emerald-600 dark:text-emerald-400' : 'text-rose-600 dark:text-rose-400'" class="text-sm mt-3 sm:mt-4">{{ secMsg }}</p>
      <button @click="handleSetQuestions" :disabled="secLoading" class="btn-brand mt-3 sm:mt-4">{{ secLoading ? '保存中...' : (hasQuestions ? '修改安全问题' : '设置安全问题') }}</button>
    </section>

    <!-- 退出登录：顶栏在手机端收起了退出按钮，这里提供统一入口。
         飞牛授权的只是身份来源，不代表应用会话不能结束——网关域上同样可以退出，
         退出后想回来再点一次登录页的「使用飞牛 NAS 登录」即可。 -->
    <section class="surface rounded-2xl p-4 sm:p-7">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div>
          <h2 class="text-base sm:text-lg font-bold text-foreground">退出登录</h2>
          <p class="mt-0.5 sm:mt-1 text-[13px] sm:text-sm text-muted-foreground">
            {{ gatewaySession ? '退出后需要重新登录才能访问你的提醒。飞牛账号不会退出，下次点「使用飞牛 NAS 登录」即可回来。' : '退出后需要重新输入账号密码才能访问你的提醒。' }}
          </p>
        </div>
        <button class="btn-ghost shrink-0 !text-rose-600 dark:!text-rose-400" @click="handleLogout">退出登录</button>
      </div>
    </section>

    <ConfirmDialog
      v-model="logoutVisible"
      danger
      title="退出登录？"
      message="退出后需要重新输入账号密码才能访问你的提醒。"
      confirm-text="退出登录"
      loading-text="退出中…"
      :loading="logoutLoading"
      @confirm="confirmLogout"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth'
import { changePassword, regenerateAPIKey } from '../api/auth'
import { getSecurityQuestions, setSecurityQuestions } from '../api/security'
import { passwordValidationError } from '../utils/password'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import { onFnOSGatewayOrigin } from '../utils/gateway'

const authStore = useAuthStore()
const router = useRouter()
const userInitial = computed(() => (authStore.user?.username || 'U').charAt(0).toUpperCase())
// 网关域：文案要说清楚「退出的是本应用，不是飞牛账号」，但退出按钮一样要有——
// 飞牛授权只是身份来源，和第三方登录一样不应该替用户决定应用会话的生死。
const gatewaySession = onFnOSGatewayOrigin()

// 退出确认走应用内浮层（见 ConfirmDialog），不再用原生 confirm。
const logoutVisible = ref(false)
const logoutLoading = ref(false)

function handleLogout() {
  logoutVisible.value = true
}

async function confirmLogout() {
  logoutLoading.value = true
  // 先让服务端记下这次主动登出（网关域上只清本地会被网关注入身份认回来）。
  await authStore.endSession()
  await router.push('/login')
  logoutLoading.value = false
  logoutVisible.value = false
}

const oldPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const pwdLoading = ref(false)
const pwdMsg = ref('')
const pwdSuccess = ref(false)

const hasQuestions = ref(false)
const qa = reactive([
  { q: '', a: '' },
  { q: '', a: '' },
  { q: '', a: '' },
])
const secLoading = ref(false)
const secMsg = ref('')
const secSuccess = ref(false)

const apiKey = ref('')
const apiKeyLoading = ref(false)
const apiKeyMsg = ref('')
const apiKeySuccess = ref(false)

onMounted(async () => {
  try {
    const res = await getSecurityQuestions()
    if (res.data?.code === 0 && res.data.data?.has_questions) {
      hasQuestions.value = true
      qa[0].q = res.data.data.question1 || ''
      qa[1].q = res.data.data.question2 || ''
      qa[2].q = res.data.data.question3 || ''
    }
  } catch {}
})

async function handleChangePassword() {
  const validationError = passwordValidationError(newPassword.value)
  if (validationError) {
    pwdMsg.value = validationError
    pwdSuccess.value = false
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    pwdMsg.value = '两次密码不一致'
    pwdSuccess.value = false
    return
  }
  pwdLoading.value = true
  pwdMsg.value = ''
  try {
    const res = await changePassword(oldPassword.value, newPassword.value)
    if (res.data?.code === 0) {
      pwdMsg.value = '密码修改成功，请重新登录'
      pwdSuccess.value = true
      oldPassword.value = ''
      newPassword.value = ''
      confirmPassword.value = ''
      setTimeout(() => {
        void authStore.endSession()
        router.push('/login')
      }, 800)
    } else {
      pwdMsg.value = res.data?.message || '修改失败'
      pwdSuccess.value = false
    }
  } catch (err: any) {
    pwdMsg.value = err.response?.data?.message || '网络错误'
    pwdSuccess.value = false
  } finally {
    pwdLoading.value = false
  }
}

async function handleSetQuestions() {
  secLoading.value = true
  secMsg.value = ''
  try {
    const res = await setSecurityQuestions(qa[0].q, qa[0].a, qa[1].q, qa[1].a, qa[2].q, qa[2].a)
    if (res.data?.code === 0) {
      secMsg.value = hasQuestions.value ? '安全问题修改成功' : '安全问题设置成功'
      secSuccess.value = true
      hasQuestions.value = true
      authStore.refreshSecurityQuestions()
    } else {
      secMsg.value = res.data?.message || '设置失败'
      secSuccess.value = false
    }
  } catch (err: any) {
    secMsg.value = err.response?.data?.message || '网络错误'
    secSuccess.value = false
  } finally {
    secLoading.value = false
  }
}

async function handleRegenerateApiKey() {
  apiKeyLoading.value = true
  apiKeyMsg.value = ''
  try {
    const res = await regenerateAPIKey()
    if (res.data?.code === 0) {
      apiKey.value = res.data.data?.api_key || res.data.data?.apiKey || ''
      apiKeyMsg.value = 'API Key 已重新生成'
      apiKeySuccess.value = true
    } else {
      apiKeyMsg.value = res.data?.message || '生成失败'
      apiKeySuccess.value = false
    }
  } catch (err: any) {
    apiKeyMsg.value = err.response?.data?.message || '网络错误'
    apiKeySuccess.value = false
  } finally {
    apiKeyLoading.value = false
  }
}

function copyApiKey() {
  navigator.clipboard.writeText(apiKey.value)
  apiKeyMsg.value = '已复制到剪贴板'
  apiKeySuccess.value = true
  setTimeout(() => { apiKeyMsg.value = '' }, 2000)
}
</script>
