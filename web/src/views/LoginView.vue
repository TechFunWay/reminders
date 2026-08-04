<template>
  <AuthShell title="登录" subtitle="输入账号信息以访问控制台">
    <form @submit.prevent="handleLogin" class="space-y-5">
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
        {{ loading ? '登录中...' : '登录' }}
      </button>
    </form>

    <template #footer v-if="authStore.allowRegister">
      还没有账号？
      <router-link to="/register" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">立即注册</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { login } from '../api/auth'
import { useAuthStore } from '../stores/auth'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'

const router = useRouter()
const authStore = useAuthStore()

const username = ref('')
const password = ref('')
const loading = ref(false)
const errorMsg = ref('')

async function handleLogin() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await login(username.value, password.value)
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
</script>
