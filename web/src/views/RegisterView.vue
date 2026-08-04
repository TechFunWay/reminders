<template>
  <AuthShell title="创建账号" subtitle="注册一个新账号以访问控制台">
    <!-- register disabled -->
    <div v-if="disabled" class="flex flex-col items-center text-center gap-3 rounded-2xl border border-white/10 bg-white/[0.03] px-6 py-10">
      <span class="w-12 h-12 rounded-full bg-amber-400/15 text-amber-300 flex items-center justify-center">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
      </span>
      <p class="text-sm text-white/70">注册功能已关闭，请联系管理员</p>
    </div>

    <form v-else @submit.prevent="handleRegister" class="space-y-5">
      <div v-if="authStore.setupRequired" class="flex flex-col gap-2 rounded-2xl border border-brand-400/30 bg-brand-400/10 px-5 py-4 text-sm">
        <div class="flex items-center gap-2 font-semibold text-brand-300">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          您是首个用户
        </div>
        <p class="text-white/80">注册后该账号将自动成为管理员，拥有全部管理权限。</p>
      </div>

      <AuthField v-model="username" label="用户名" autocomplete="username" required placeholder="请输入用户名" autofocus>
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
        </template>
      </AuthField>

      <AuthField v-model="password" label="密码" type="password" autocomplete="new-password" required placeholder="请输入密码">
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="9" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V8a4 4 0 118 0v3"/></svg>
        </template>
      </AuthField>

      <AuthField v-model="confirmPassword" label="确认密码" type="password" autocomplete="new-password" required placeholder="请再次输入密码">
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
        </template>
      </AuthField>

      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="errorMsg" class="flex items-center gap-2 text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">
          <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 9v2m0 4h.01M5.07 19h13.86a2 2 0 001.74-3L13.74 4a2 2 0 00-3.48 0L3.34 16a2 2 0 001.73 3z"/></svg>
          {{ errorMsg }}
        </div>
      </transition>
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="successMsg" class="flex items-center gap-2 text-sm text-emerald-300 bg-emerald-500/10 border border-emerald-500/25 rounded-xl px-3.5 py-2.5">
          <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          {{ successMsg }}
        </div>
      </transition>

      <button type="submit" :disabled="loading" class="btn-premium">
        <svg v-if="loading" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
        {{ loading ? '注册中...' : '注册' }}
      </button>
    </form>

    <template #footer v-if="!authStore.setupRequired">
      已有账号？
      <router-link to="/login" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">立即登录</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { register } from '../api/auth'
import { getPublicConfigs } from '../api/config'
import { useAuthStore } from '../stores/auth'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'
import { passwordValidationError } from '../utils/password'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')
const disabled = ref(false)

onMounted(async () => {
  try {
    const res = await getPublicConfigs()
    if (res.data?.code === 0) {
      disabled.value = res.data.data?.allow_register === 'false' && !authStore.setupRequired
    }
  } catch {}
})

async function handleRegister() {
  const validationError = passwordValidationError(password.value)
  if (validationError) {
    errorMsg.value = validationError
    return
  }
  if (password.value !== confirmPassword.value) {
    errorMsg.value = '两次密码不一致'
    return
  }
  loading.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const res = await register(username.value, password.value)
    if (res.data?.code === 0) {
      if (authStore.setupRequired) {
        successMsg.value = '注册成功，正在进入控制台'
        const token = res.data.data?.token
        const user = res.data.data?.user
        if (token && user) {
          authStore.setToken(token)
          authStore.setUser(user)
          authStore.resetInit()
          setTimeout(() => router.push('/admin'), 800)
        } else {
          successMsg.value = ''
          errorMsg.value = '注册成功但无法自动登录，请手动登录'
        }
      } else {
        successMsg.value = '注册成功，即将跳转登录页'
        setTimeout(() => router.push('/login'), 1500)
      }
    } else {
      errorMsg.value = res.data?.message || '注册失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}
</script>
