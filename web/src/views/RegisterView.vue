<template>
  <AuthShell :title="pageTitle" :subtitle="pageSubtitle">
    <!-- register disabled -->
    <div v-if="disabled" class="flex flex-col items-center text-center gap-3 rounded-2xl border border-white/10 bg-white/[0.03] px-6 py-10">
      <span class="w-12 h-12 rounded-full bg-amber-400/15 text-amber-300 flex items-center justify-center">
        <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z"/></svg>
      </span>
      <p class="text-sm text-white/70">注册功能已关闭，请联系管理员</p>
    </div>

    <form v-else @submit.prevent="handleRegister" class="space-y-5">
      <div v-if="isFnOSBinding && !authStore.setupRequired" class="rounded-2xl border border-brand-400/30 bg-brand-400/10 px-5 py-4 text-sm text-white/85">
        当前飞牛 NAS 用户 <span class="font-semibold text-brand-200">{{ fnosUsername || '已登录用户' }}</span> 尚未绑定应用账号。
        <template v-if="fnosMode === 'bind'">请输入电脑端正在使用的应用账号密码；绑定后，两端会使用同一份提醒和通知方式。</template>
        <template v-else>创建后将成为一个数据独立的新账号；如果电脑端已有数据，请改为绑定已有账号。</template>
      </div>
      <div v-if="authStore.setupRequired" class="flex flex-col gap-2 rounded-2xl border border-brand-400/30 bg-brand-400/10 px-5 py-4 text-sm">
        <div class="flex items-center gap-2 font-semibold text-brand-300">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"/></svg>
          {{ isFnOSBinding ? '首次使用：使用飞牛 NAS 创建管理员' : '首次使用：创建管理员账号' }}
        </div>
        <p v-if="isFnOSBinding" class="text-white/80">当前还没有任何用户。创建后该账号将自动成为管理员，并立即绑定当前飞牛 NAS 用户，之后可直接一键登录。</p>
        <p v-else-if="fnosEnabled" class="text-white/80">当前还没有任何用户。创建后该账号将自动成为管理员；如需绑定飞牛 NAS 一键登录，可点击下方「使用飞牛 NAS 登录」。</p>
        <p v-else class="text-white/80">当前还没有任何用户。创建后该账号将自动成为管理员。</p>
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

      <AuthField v-if="!isFnOSBinding || fnosMode === 'register'" v-model="confirmPassword" label="确认密码" type="password" autocomplete="new-password" required placeholder="请再次输入密码">
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
        {{ loading ? '处理中...' : submitLabel }}
      </button>

      <button v-if="isFnOSBinding && !authStore.setupRequired" type="button" :disabled="loading" @click="fnosMode = fnosMode === 'register' ? 'bind' : 'register'" class="w-full text-sm font-medium text-brand-300 hover:text-brand-200 transition-colors">
        {{ fnosMode === 'register' ? '已有应用账号？验证并绑定' : '没有应用账号？创建并绑定' }}
      </button>

      <button v-else-if="isFnOSBinding && authStore.setupRequired" type="button" :disabled="loading" @click="backToPlainSetup" class="w-full text-sm font-medium text-white/60 hover:text-white/85 transition-colors">
        不使用飞牛 NAS，改用用户名密码创建
      </button>

      <button v-else-if="fnosEnabled" type="button" :disabled="loading" @click="handleFnOSAuthorize" class="w-full min-h-10 inline-flex items-center justify-center rounded-xl border border-white/15 bg-transparent px-4 py-3 text-sm font-semibold text-white transition-colors hover:bg-white/[0.08] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-brand-300/70 disabled:cursor-not-allowed disabled:opacity-60">
        {{ loading ? '正在获取飞牛账号…' : '使用飞牛 NAS 登录' }}
      </button>
    </form>

    <Teleport to="body">
      <transition enter-active-class="transition duration-200" enter-from-class="opacity-0" leave-active-class="transition duration-150" leave-to-class="opacity-0">
        <div v-if="showFnOSConfirm" class="fixed inset-0 z-[100] flex items-center justify-center bg-black/65 px-4 backdrop-blur-sm" @click.self="cancelFnOSConfirm">
          <section role="dialog" aria-modal="true" aria-labelledby="fnos-confirm-title" class="w-full max-w-md rounded-3xl border border-white/15 bg-[#171827] p-6 text-white shadow-2xl sm:p-8">
            <div class="mx-auto flex h-16 w-16 items-center justify-center rounded-2xl bg-gradient-to-br from-brand-500 to-violet-500 text-2xl font-bold shadow-lg shadow-brand-500/25">
              {{ fnosConfirmUsername.slice(0, 1) || '飞' }}
            </div>
            <h2 id="fnos-confirm-title" class="mt-5 text-center text-2xl font-bold">确认使用飞牛 NAS 创建管理员</h2>
            <p class="mt-2 text-center text-sm leading-6 text-white/65">“提醒事项”正在请求使用下面的飞牛 NAS 账号</p>
            <div class="mt-6 flex items-center justify-center gap-3 rounded-2xl border border-white/10 bg-white/[0.06] px-4 py-4">
              <span class="flex h-10 w-10 items-center justify-center rounded-full bg-brand-400/25 font-semibold text-brand-100">{{ fnosConfirmUsername.slice(0, 1) || '飞' }}</span>
              <span class="font-semibold">{{ fnosConfirmUsername || '当前飞牛 NAS 用户' }}</span>
            </div>
            <button type="button" :disabled="loading" @click="confirmFnOSAccount" class="btn-premium mt-6">
              使用该账号
            </button>
            <button v-if="!fnosMobile" type="button" :disabled="loading" @click="switchFnOSAccount" class="mt-3 w-full rounded-xl px-4 py-3 text-sm font-semibold text-brand-200 transition-colors hover:bg-white/[0.06] hover:text-brand-100 disabled:opacity-60">
              使用其他飞牛账号
            </button>
            <p v-else class="mt-3 text-xs leading-5 text-white/45">如需更换飞牛账号，请在飞牛 App 中切换后重新打开本应用。</p>
          </section>
        </div>
      </transition>
    </Teleport>

    <template #footer v-if="!authStore.setupRequired">
      已有账号？
      <router-link to="/login" class="font-semibold text-brand-300 hover:text-brand-200 transition-colors">立即登录</router-link>
    </template>
  </AuthShell>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { register, bindFnOSAccount, fnosLogin } from '../api/auth'
import { useAuthStore } from '../stores/auth'
import { resolveFnOSGatewayEntry, startFnOSAccountSwitch, startFnOSAuthorize, startFnOSFullPageAuthorize, tryIssueFnOSTicket, fnOSMobileClient } from '../utils/fnos-auth'
import { SK } from '../utils/storage-keys'
import AuthShell from '../components/auth/AuthShell.vue'
import AuthField from '../components/auth/AuthField.vue'
import { passwordValidationError } from '../utils/password'

const router = useRouter()
const route = useRoute()
const authStore = useAuthStore()

const username = ref(typeof route.query.fnos_username === 'string' ? route.query.fnos_username : '')
const password = ref('')
const confirmPassword = ref('')
const loading = ref(false)
const errorMsg = ref('')
const successMsg = ref('')
const disabled = ref(false)
const fnosMode = ref<'register' | 'bind'>(route.query.fnos_mode === 'bind' ? 'bind' : 'register')
// 按钮与绑定态的显隐以服务端运行形态为准（authStore.fnosApp，随启动引导
// 取回）：非「-fnos-app」部署（裸二进制 / Docker）一律藏掉，编译期标志只是
// 引导返回前的兜底默认。?fnos=bind 只会由飞牛授权流程带回，非飞牛部署下
// 即便被人手工拼进地址也不该进入绑定界面。
const isFnOSBinding = computed(() => authStore.fnosApp && route.query.fnos === 'bind')
const fnosEnabled = computed(() => authStore.fnosApp)
const fnosUsername = computed(() => typeof route.query.fnos_username === 'string' ? route.query.fnos_username : '')
// 页内确认框：弹窗不可用（飞牛手机 App 网页视图）时就地签票后的确认入口；
// 初始化（无任何用户）时路由守卫会把登录页弹回注册页，确认框必须在本页完成。
const showFnOSConfirm = ref(false)
const fnosConfirmUsername = ref('')
// 手机端不提供「切换飞牛账号」（见 fnOSMobileClient 注释），改为提示在飞牛 App 内切换。
const fnosMobile = fnOSMobileClient()
const pageTitle = computed(() => authStore.setupRequired
  ? (isFnOSBinding.value ? '使用飞牛 NAS 创建管理员' : '创建管理员账号')
  : (isFnOSBinding.value ? '绑定飞牛 NAS 账号' : '创建账号'))
const pageSubtitle = computed(() => authStore.setupRequired
  ? (isFnOSBinding.value ? '创建首个管理员账号，并绑定当前飞牛 NAS 用户' : '首次使用，请先完成管理员初始化')
  : (isFnOSBinding.value ? '创建或绑定应用账号，之后即可使用飞牛 NAS 一键登录' : '注册一个新账号以访问控制台'))
const submitLabel = computed(() => {
  if (authStore.setupRequired) return isFnOSBinding.value ? '创建管理员并绑定飞牛 NAS' : '创建管理员'
  if (isFnOSBinding.value) return fnosMode.value === 'register' ? '创建并绑定' : '验证并绑定'
  return '注册'
})

onMounted(async () => {
  // 是否允许注册已随启动引导取回（authStore.allowRegister），不再单独发请求。
  // 初始化创建管理员与飞牛绑定场景始终可用，其余按公开配置判断。
  disabled.value = authStore.allowRegister === false && !authStore.setupRequired && !isFnOSBinding.value
  // 每次都重新解析一次入口：应用就跑在网关域时它立即返回当前源地址并写入
  // localStorage，把服务端登记里可能残留的、来自另一个网络的旧入口纠正掉。
  if (fnosEnabled.value) {
    await resolveFnOSGatewayEntry({ servicePort: servicePort.value, entry: gatewayEntry.value })
  }
  // 整页授权流程返回的场景：票据已由 main.ts 从 hash 接到 sessionStorage；
  // 确认动作已在授权页完成，这里拿票据直接继续（登录换取绑定上下文）。
  // 否则用户会在注册页看到「什么都没发生」，票据也白白过期。
  const existingTicket = sessionStorage.getItem(SK.fnosTicket)
  if (fnosEnabled.value && existingTicket && !authStore.isAuthenticated) {
    if (isFnOSBinding.value || authStore.setupRequired) {
      errorMsg.value = ''
      confirmFnOSAccount()
    } else {
      clearFnOSTicket()
    }
  }
})

// 注册页上的飞牛入口：优先弹窗打开网关授权页（fnos-entry.html），账号展示、
// 换账号、确认全部在弹窗里完成，本页不跳转不刷新；弹窗确认后 postMessage
// 交回一次性票据。弹窗不可用（被拦截 / 飞牛手机 App 网页视图假句柄）时由
// startFnOSAuthorize 回调兜底：网关域就地签票弹页内确认框，直连端口整页跳
// 授权页（票据经 hash 回本页，见 onMounted）。两个值都随启动引导取回，缓存
// 在 store / localStorage 里，点击时不必再等一次请求。
const servicePort = computed(() => authStore.servicePort)
const gatewayEntry = computed(() => authStore.fnosGatewayEntry)

async function handleFnOSAuthorize() {
  if (loading.value) return
  loading.value = true
  errorMsg.value = ''
  const gatewayURL = await resolveFnOSGatewayEntry({ servicePort: servicePort.value, entry: gatewayEntry.value })
  loading.value = false
  if (!gatewayURL) {
    errorMsg.value = '请先从飞牛桌面打开本应用，再使用飞牛授权登录'
    return
  }
  startFnOSAuthorize(gatewayURL, {
    onTicket: (ticket, username) => {
      storeFnOSTicket(ticket, username)
      if (authStore.setupRequired) {
        confirmFnOSAccount()
      } else {
        // 普通注册页：确认动作已在授权页完成，转登录页拿票据直接登录。
        router.replace({ name: 'Login' })
      }
    },
    onUnavailable: () => fallbackFnOSAuthorize(gatewayURL),
  })
}

function storeFnOSTicket(ticket: string, username: string) {
  sessionStorage.setItem(SK.fnosTicket, ticket)
  if (username) sessionStorage.setItem(SK.fnosTicketUsername, username)
  else sessionStorage.removeItem(SK.fnosTicketUsername)
  sessionStorage.removeItem(SK.fnosEntryError)
}

// 弹窗不可用时的兜底：网关域（飞牛桌面入口 / 手机 App 的页面本来就在网关域）
// 就地签票，拿到票据即弹页内确认框（初始化场景必须在本页完成绑定创建）；只有
// 直连端口就地签票必然 401，才整页跳授权页。
async function fallbackFnOSAuthorize(gatewayURL: string) {
  loading.value = true
  const issued = await tryIssueFnOSTicket()
  if (issued) {
    loading.value = false
    storeFnOSTicket(issued.ticket, issued.username)
    if (authStore.setupRequired) {
      fnosConfirmUsername.value = issued.username
      showFnOSConfirm.value = true
    } else {
      router.replace({ name: 'Login' })
    }
    return
  }
  // 网关域也签不到票（直连端口）：整页跳授权页。跳转前探测可达性——入口可能来自
  // 另一个网络，打不开时就地提示，不把用户送到浏览器的「无法访问页面」。
  const opened = await startFnOSFullPageAuthorize(gatewayURL)
  loading.value = false
  if (!opened) {
    errorMsg.value = '当前网络无法打开飞牛授权页，请检查网络后重试，或改用用户名密码注册'
  }
}

// 页内确认框里的「使用其他飞牛账号」：弹窗打开飞牛登录页换号，登录成功后飞牛
// 登录页回跳到【同源】授权页取新票据，再 postMessage 回本页完成绑定/创建——
// 应用页全程不跳转。登录页不可达（飞牛 App 沙箱源）时就地提示，不整页跳。
async function switchFnOSAccount() {
  if (loading.value) return
  let gatewayURL = localStorage.getItem(SK.fnosGatewayUrl) || gatewayEntry.value
  if (!gatewayURL || !gatewayURL.includes('/app/')) {
    gatewayURL = (await resolveFnOSGatewayEntry({ servicePort: servicePort.value, entry: gatewayEntry.value })) || ''
  }
  if (!gatewayURL || !gatewayURL.includes('/app/')) {
    showFnOSConfirm.value = false
    clearFnOSTicket()
    errorMsg.value = '无法定位飞牛桌面入口，请从飞牛桌面重新打开本应用'
    return
  }
  clearFnOSTicket()
  showFnOSConfirm.value = false
  errorMsg.value = ''
  loading.value = true
  const started = await startFnOSAccountSwitch(gatewayURL, servicePort.value, {
    onTicket: (ticket, username) => {
      storeFnOSTicket(ticket, username)
      confirmFnOSAccount()
    },
  })
  loading.value = false
  if (!started) {
    errorMsg.value = '当前环境无法打开飞牛登录页，请在飞牛 App 中切换飞牛账号后重新打开本应用'
  }
}

// 授权页（弹窗 / 整页）、页内确认框里确认使用该飞牛账号后走到这里：带上票据
// 调用一键登录，预期返回 binding_required（当前还没有任何应用账号）。据此转成
// fnos=bind 的注册绑定模式并预填用户名。
async function confirmFnOSAccount() {
  loading.value = true
  errorMsg.value = ''
  showFnOSConfirm.value = false
  try {
    const res = await fnosLogin()
    if (res.data?.code !== 0) {
      clearFnOSTicket()
      errorMsg.value = res.data?.message || '飞牛一键登录失败'
      return
    }
    const data = res.data.data
    if (data?.binding_required) {
      router.replace({
        query: {
          fnos: 'bind',
          fnos_username: data.fnos_username || fnosConfirmUsername.value,
          fnos_mode: 'register',
        },
      })
      if (data.fnos_username && !username.value) username.value = data.suggested_username || data.fnos_username
      fnosConfirmUsername.value = ''
      return
    }
    // 理论上初始化阶段不会走到这里（有账号时不再 setupRequired）；兜底按
    // 已登录处理。
    clearFnOSTicket()
    if (data?.token && data?.user) {
      authStore.setToken(data.token)
      authStore.setUser(data.user)
      authStore.resetInit()
      router.push('/admin')
    }
  } catch (err: any) {
    clearFnOSTicket()
    errorMsg.value = err.response?.data?.message || '飞牛一键登录失败'
  } finally {
    loading.value = false
  }
}

function cancelFnOSConfirm() {
  showFnOSConfirm.value = false
  clearFnOSTicket()
  fnosConfirmUsername.value = ''
}

function clearFnOSTicket() {
  sessionStorage.removeItem(SK.fnosTicket)
  sessionStorage.removeItem(SK.fnosTicketUsername)
}

// 初始化阶段从「飞牛绑定创建」退回「用户名密码创建」：清掉查询参数与尚未
// 使用的授权票据，避免普通创建时误带飞牛绑定。
function backToPlainSetup() {
  clearFnOSTicket()
  showFnOSConfirm.value = false
  fnosConfirmUsername.value = ''
  router.replace({ query: {} })
}

async function handleRegister() {
  if (!isFnOSBinding.value || fnosMode.value === 'register') {
    const validationError = passwordValidationError(password.value)
    if (validationError) {
      errorMsg.value = validationError
      return
    }
  }
  if ((!isFnOSBinding.value || fnosMode.value === 'register') && password.value !== confirmPassword.value) {
    errorMsg.value = '两次密码不一致'
    return
  }
  loading.value = true
  errorMsg.value = ''
  successMsg.value = ''
  try {
    if (isFnOSBinding.value) {
      const res = await bindFnOSAccount(fnosMode.value, username.value, password.value)
      if (res.data?.code === 0) {
        sessionStorage.removeItem(SK.fnosTicket)
        sessionStorage.removeItem(SK.fnosTicketUsername)
        authStore.setToken(res.data.data.token)
        authStore.setUser(res.data.data.user)
        authStore.resetInit()
        router.push('/admin')
      } else {
        errorMsg.value = res.data?.message || '绑定失败'
      }
      return
    }
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
