<template>
  <AuthShell title="登录" :subtitle="fnosBindingRequired ? '登录已有应用账号并绑定当前飞牛 NAS 用户' : '输入账号信息以访问控制台'">
    <form @submit.prevent="handleLogin" class="space-y-5">
      <div v-if="fnosBindingRequired" class="rounded-2xl border border-brand-400/30 bg-brand-400/10 px-5 py-4 text-sm leading-6 text-white/85">
        当前飞牛 NAS 用户 <span class="font-semibold text-brand-200">{{ fnosUsername || '已登录用户' }}</span> 尚未绑定。
        系统中已有应用账号，请登录现有账号；验证成功后会自动完成绑定，并继续使用原来的提醒和通知方式。
      </div>

      <AuthField v-model="username" label="用户名" autocomplete="username" required placeholder="请输入用户名" autofocus>
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
        </template>
      </AuthField>

      <AuthField v-model="password" label="密码" type="password" autocomplete="current-password" required placeholder="请输入密码">
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="9" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V8a4 4 0 118 0v3"/></svg>
        </template>
        <template #labelRight>
          <router-link to="/forgot-password" class="text-xs font-medium text-brand-300 hover:text-brand-200 transition-colors">忘记密码？</router-link>
        </template>
      </AuthField>

      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="errorMsg" class="flex items-center gap-2 text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">
          <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M5.07 19h13.86a2 2 0 001.74-3L13.74 4a2 2 0 00-3.48 0L3.34 16a2 2 0 001.73 3z"/></svg>
          {{ errorMsg }}
        </div>
      </transition>

      <button type="submit" :disabled="loading" class="btn-premium">
        <svg v-if="loading" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
        {{ loading ? (fnosBindingRequired ? '登录并绑定中...' : '登录中...') : (fnosBindingRequired ? '登录并绑定' : '登录') }}
      </button>

      <button v-if="fnosEnabled" type="button" :disabled="loading" @click="handleFnOSLogin" class="w-full min-h-12 inline-flex items-center justify-center rounded-xl border border-white/15 bg-transparent px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-white/[0.08] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-300/70 disabled:cursor-not-allowed disabled:opacity-60">
        {{ loading ? '正在获取飞牛账号…' : '使用飞牛 NAS 登录' }}
      </button>
    </form>

    <Teleport to="body">
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="showFnOSConfirm" class="fixed inset-0 z-[100] flex items-center justify-center bg-black/65 px-4 backdrop-blur-sm" @click.self="closeFnOSConfirm">
          <section role="dialog" aria-modal="true" aria-labelledby="fnos-confirm-title" class="w-full max-w-md rounded-3xl border border-white/15 bg-[#171827] p-6 text-white shadow-2xl sm:p-8">
            <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-500 text-2xl font-bold shadow-lg shadow-brand-500/25">
              {{ fnosConfirmUsername.slice(0, 1) || '飞' }}
            </div>
            <h2 id="fnos-confirm-title" class="mt-5 text-center text-2xl font-bold">确认使用飞牛 NAS 登录</h2>
            <p class="mt-2 text-center text-sm leading-6 text-white/65">“提醒事项”正在请求使用下面的飞牛 NAS 账号登录</p>
            <div class="mt-6 flex items-center justify-center gap-3 rounded-2xl border border-white/10 bg-white/[0.06] px-4 py-4">
              <span class="flex h-10 w-10 items-center justify-center rounded-full bg-brand-400/25 font-semibold text-brand-100">{{ fnosConfirmUsername.slice(0, 1) || '飞' }}</span>
              <span class="font-semibold">{{ fnosConfirmUsername || '当前飞牛 NAS 用户' }}</span>
            </div>
            <button type="button" :disabled="loading" @click="confirmFnOSLogin" class="btn-premium mt-6">
              {{ loading ? '正在登录…' : '确认登录' }}
            </button>
            <button type="button" :disabled="loading" @click="switchFnOSAccount" class="mt-3 w-full rounded-xl px-4 py-3 text-sm font-semibold text-brand-200 transition-colors hover:bg-white/[0.06] hover:text-brand-100 disabled:opacity-60">
              使用其他飞牛账号
            </button>
            <button type="button" :disabled="loading" @click="closeFnOSConfirm" class="mt-1 w-full rounded-xl px-4 py-2 text-sm text-white/55 transition-colors hover:text-white/80 disabled:opacity-60">取消</button>
          </section>
        </div>
      </transition>
    </Teleport>

    <template #footer v-if="authStore.allowRegister">
      还没有账号？
      <router-link to="/register" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">立即注册</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { bindFnOSAccount, getFnOSIdentity, login, fnosLogin } from '../api/auth'
import { useAuthStore } from '../stores/auth'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')
const fnosBindingRequired = ref(false)
const fnosUsername = ref('')
const fnosConfirmUsername = ref('')
const showFnOSConfirm = ref(false)
const fnosEnabled = import.meta.env.VITE_FNOS_APP === 'true'

async function handleLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = fnosBindingRequired.value
      ? await bindFnOSAccount('bind', username.value, password.value)
      : await login(username.value, password.value)
    if (res.data?.code === 0) {
      authStore.setToken(res.data.data.token)
      authStore.setUser(res.data.data.user)
      router.push('/admin')
    } else {
      errorMsg.value = res.data?.message || '登录失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}

async function handleFnOSLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getFnOSIdentity()
    if (res.data?.code !== 0) {
      errorMsg.value = res.data?.message || '无法获取飞牛 NAS 账号'
      return
    }
    fnosConfirmUsername.value = res.data.data?.fnos_username || ''
    showFnOSConfirm.value = true
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '无法获取飞牛 NAS 账号'
  } finally {
    loading.value = false
  }
}

function closeFnOSConfirm() {
  if (!loading.value) {
    showFnOSConfirm.value = false
  }
}

function switchFnOSAccount() {
  const appLoginPath = `${import.meta.env.BASE_URL}login?fnos_account_switched=1`
  window.location.assign(`/login?redirect_uri=${encodeURIComponent(appLoginPath)}`)
}

async function confirmFnOSLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await fnosLogin()
    if (res.data?.code !== 0) {
      showFnOSConfirm.value = false
      errorMsg.value = res.data?.message || '飞牛一键登录失败'
      return
    }
    if (res.data.data?.binding_required) {
      showFnOSConfirm.value = false
      if (res.data.data.has_accounts || res.data.data.suggested_mode === 'bind') {
        fnosBindingRequired.value = true
        fnosUsername.value = res.data.data.fnos_username || ''
        username.value = res.data.data.suggested_username || ''
        return
      }
      router.push({
        name: 'Register',
        query: {
          fnos: 'bind',
          fnos_username: res.data.data.fnos_username || '',
          fnos_mode: 'register',
        },
      })
      return
    }
    authStore.setToken(res.data.data.token)
    authStore.setUser(res.data.data.user)
    showFnOSConfirm.value = false
    router.push('/admin')
  } catch (err: any) {
    showFnOSConfirm.value = false
    errorMsg.value = err.response?.data?.message || '飞牛一键登录失败'
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  if (fnosEnabled && route.query.fnos_account_switched === '1') {
    await router.replace({ name: 'Login' })
    await handleFnOSLogin()
  }
})
</script>
