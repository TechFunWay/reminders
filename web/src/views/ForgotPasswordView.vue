<template>
  <AuthShell title="找回密码" :subtitle="stepSubtitle">
    <!-- step indicator -->
    <div class="flex items-center gap-2 mb-7">
      <template v-for="(s, i) in 3" :key="i">
        <div
          class="h-1.5 flex-1 rounded-full transition-colors duration-500"
          :class="step >= i + 1 ? 'bg-brand-gradient' : 'bg-white/10'"
        ></div>
      </template>
    </div>

    <!-- Step 1: username -->
    <form v-if="step === 1" @submit.prevent="handleGetQuestions" class="space-y-5">
      <AuthField v-model="username" label="用户名" required placeholder="请输入用户名" autofocus>
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z"/></svg>
        </template>
      </AuthField>
      <div v-if="errorMsg" class="text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">{{ errorMsg }}</div>
      <button type="submit" :disabled="loading" class="btn-premium">
        <svg v-if="loading" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
        {{ loading ? '查询中...' : '下一步' }}
      </button>
    </form>

    <!-- Step 2: security questions -->
    <form v-else-if="step === 2" @submit.prevent="handleVerify" class="space-y-5">
      <AuthField v-model="answer1" :label="questions.question1" required placeholder="请输入答案" autofocus />
      <AuthField v-model="answer2" :label="questions.question2" required placeholder="请输入答案" />
      <AuthField v-model="answer3" :label="questions.question3" required placeholder="请输入答案" />
      <div v-if="errorMsg" class="text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">{{ errorMsg }}</div>
      <button type="submit" :disabled="loading" class="btn-premium">下一步</button>
    </form>

    <!-- Step 3: new password -->
    <form v-else @submit.prevent="handleReset" class="space-y-5">
      <AuthField v-model="newPassword" label="新密码" type="password" autocomplete="new-password" required placeholder="请输入新密码" autofocus>
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><rect x="5" y="11" width="14" height="9" rx="2" stroke-width="2"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M8 11V8a4 4 0 118 0v3"/></svg>
        </template>
      </AuthField>
      <AuthField v-model="confirmPassword" label="确认新密码" type="password" autocomplete="new-password" required placeholder="请再次输入新密码">
        <template #icon>
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
        </template>
      </AuthField>
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="errorMsg" class="text-sm text-red-300 bg-red-500/10 border border-red-500/25 rounded-xl px-3.5 py-2.5">{{ errorMsg }}</div>
      </transition>
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0 -translate-y-1" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="successMsg" class="flex items-center gap-2 text-sm text-emerald-300 bg-emerald-500/10 border border-emerald-500/25 rounded-xl px-3.5 py-2.5">
          <svg class="w-4 h-4 shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          {{ successMsg }}
        </div>
      </transition>
      <button type="submit" :disabled="loading" class="btn-premium">
        <svg v-if="loading" class="w-5 h-5 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
        {{ loading ? '重置中...' : '重置密码' }}
      </button>
    </form>

    <template #footer>
      <router-link to="/login" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">返回登录</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import { getQuestionsByUsername, verifyAnswers, verifyAndReset } from '../api/security'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'
import { passwordValidationError } from '../utils/password'

const router = useRouter()

const step = ref(1)
const username = ref('')
const answer1 = ref('')
const answer2 = ref('')
const answer3 = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')
const questions = ref<Record<string, string>>({})

const stepSubtitle = computed(() =>
  step.value === 1 ? '输入用户名以获取安全问题' : step.value === 2 ? '回答你设置的安全问题' : '设置一个新的登录密码',
)

async function handleGetQuestions() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await getQuestionsByUsername(username.value)
    if (res.data?.code === 0) {
      questions.value = res.data.data
      step.value = 2
    } else {
      errorMsg.value = res.data?.message || '获取安全问题失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}

async function handleVerify() {
  loading.value = true
  errorMsg.value = ''
  try {
    const res = await verifyAnswers(username.value, answer1.value, answer2.value, answer3.value)
    if (res.data?.code === 0) {
      step.value = 3
    } else {
      errorMsg.value = res.data?.message || '安全问题验证失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}

async function handleReset() {
  const validationError = passwordValidationError(newPassword.value)
  if (validationError) {
    errorMsg.value = validationError
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    errorMsg.value = '两次密码不一致'
    return
  }
  loading.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    const res = await verifyAndReset(username.value, answer1.value, answer2.value, answer3.value, newPassword.value)
    if (res.data?.code === 0) {
      successMsg.value = '密码重置成功，即将跳转登录页'
      setTimeout(() => router.push('/login'), 1500)
    } else {
      errorMsg.value = res.data?.message || '重置失败'
    }
  } catch (err: any) {
    errorMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    loading.value = false
  }
}
</script>
