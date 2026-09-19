import { defineStore } from 'pinia'
import { ref, computed, watch } from 'vue'
import { checkAuth, checkSetupRequired as apiCheckSetupRequired, logout as apiLogout } from '../api/auth'
import { getBootstrap, type BootstrapData } from '../api/bootstrap'
import { getUserConfigMeta } from '../api/config'
import { getSecurityQuestions } from '../api/security'
import { useThemeStore } from './theme'
import { withTimeout } from '../utils/async'
import { setBootHint } from '../utils/boot'
import { SK } from '../utils/storage-keys'
import { onFnOSGatewayOrigin } from '../utils/gateway'

// 启动阶段单次请求的上限：超过就按默认值继续渲染，绝不无限等。取 4 秒是为
// 了容忍低端 NAS 首帧较慢，又不至于让弱网下一直停在首屏骨架上——启动骨架
// 已经替掉了白屏，用户对「稍等」的容忍度远高于对「一片空白」的。
const STARTUP_TIMEOUT_MS = 4000
// 服务可能还在初始化（监听已就绪、建库迁移未完成时返回 503 + Retry-After），
// 这时重试比直接按默认值渲染更正确：默认值会把首次安装的用户带到登录页。
const STARTUP_MAX_ATTEMPTS = 3
const STARTUP_RETRY_DELAY_MS = 1200

function delay(ms: number) {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem(SK.token) || '')
  const user = ref<any>(null)
  const requireLogin = ref(true)
  const allowRegister = ref(true)
  const setupRequired = ref(false)
  const siteTitle = ref('提醒事项')
  // 飞牛部署下授权登录要用的两个信息，随启动引导一起取回，登录/注册页不再
  // 单独查一次版本信息（弱网下这一次往返就够用户点两次按钮了）。
  const servicePort = ref('')
  const fnosGatewayEntry = ref('')
  // 后端是否真以飞牛应用模式（-fnos-app）运行，随启动引导取回。编译期
  // VITE_FNOS_APP 只作为引导返回前的兜底默认：同一套飞牛版前端产物也会被
  // 裸二进制 / Docker 直接跑起来，那种部署下「使用飞牛 NAS 登录」点了只会
  // 得到「请先从飞牛桌面打开本应用」的死路，必须以服务端运行形态为准藏掉。
  const fnosApp = ref(import.meta.env.VITE_FNOS_APP === 'true')
  // 当前登录态从哪来，由服务端在 bootstrap 里判定（只有服务端知道这次请求是
  // 不是走网关 socket、以及有没有应用自己的凭证）：
  //   gateway —— 仅凭网关注入的 NAS 身份成立，会话实际归 NAS 所有，NAS 那侧
  //              退出后应用必须跟着退出（服务端会拒绝，此时自动登出并回登录页）；
  //   app     —— 应用自己签发的会话（显式登录或直连端口），可以独立退出，
  //              也完全不需要跟随 NAS。
  // 关键：两种来源都必须能点「退出登录」。飞牛授权只是身份来源，和 QQ/微信
  // 登录第三方应用一样，授权方还登录着不代表应用会话不能结束——把两者绑成
  // 一个东西正是 2026-09-17 用户报的「飞牛登录的应用退不出去」。
  const sessionSource = ref<'app' | 'gateway'>('app')
  const followsFnOSSession = computed(() => onFnOSGatewayOrigin() && sessionSource.value === 'gateway')
  watch(siteTitle, (t) => { document.title = t }, { immediate: true })
  const hasSecurityQuestions = ref(true)
  const DISMISS_KEY = SK.securityPromptDismissed
  const DISMISS_TTL = 60 * 60 * 1000 // 1 hour
  const securityPromptDismissed = ref((() => {
    const ts = localStorage.getItem(DISMISS_KEY)
    if (!ts) return false
    return Date.now() - Number(ts) < DISMISS_TTL
  })())
  let initialized = false
  let initPromise: Promise<void> | null = null

  // 网关域上的登录态来自网关注入的 X-Trim-* 身份（服务端解析成已绑定的应用
  // 账号），此时本地没有也不需要有 JWT；直连端口仍然靠 token。所以判断登录态只
  // 看有没有用户，token 只决定「要不要往请求里带」。
  const isAuthenticated = computed(() => !!user.value)
  const isAdmin = computed(() => user.value?.role === 'admin')

  function clearSession() {
    token.value = ''
    user.value = null
    localStorage.removeItem(SK.token)
  }

  function applyBootstrap(data: BootstrapData) {
    setupRequired.value = !!data.setup_required
    const configs = data.configs || {}
    requireLogin.value = configs.require_login !== 'false'
    allowRegister.value = configs.allow_register !== 'false'
    if (configs.site_title) siteTitle.value = configs.site_title
    if (data.service_port) servicePort.value = data.service_port
    // 服务端运行形态是按钮显隐的唯一权威；字段缺席（旧后端）时保留编译期
    // 兜底默认，不做无据降级。
    if (typeof data.fnos_app === 'boolean') fnosApp.value = data.fnos_app
    // 服务端只在本应用确实靠网关注入身份登录时才回 gateway；其余情况（直连
    // 端口、应用自己的令牌）都算应用会话，绝不跟随 NAS 退出。
    sessionSource.value = data.session_source === 'gateway' ? 'gateway' : 'app'
    const entry = data.fnos_gateway_entry
    if (typeof entry === 'string' && entry.includes(import.meta.env.BASE_URL)) {
      fnosGatewayEntry.value = entry
      try { localStorage.setItem(SK.fnosGatewayUrl, entry) } catch {}
    }
  }

  async function init() {
    if (initialized) return
    if (initPromise) return initPromise

    initPromise = (async () => {
      // 启动只发一次引导请求：公开配置、初始化状态、登录态校验都在里面。
      // 请求并行、串行都改不了弱网的往返次数，合并接口才是真正省掉的那两跳。
      let data: BootstrapData | null = null
      for (let attempt = 0; attempt < STARTUP_MAX_ATTEMPTS && !data; attempt++) {
        if (attempt > 0) {
          setBootHint('服务正在启动，请稍候…')
          await delay(STARTUP_RETRY_DELAY_MS)
        }
        const res = await withTimeout(getBootstrap(), STARTUP_TIMEOUT_MS, null)
        if (!res) continue
        if (res.data?.code === 0) data = res.data.data as BootstrapData
        // 服务已应答（含错误应答）：重试不会改变结果。
        break
      }

      if (data) {
        applyBootstrap(data)
        if (setupRequired.value && token.value) {
          clearSession()
          return
        }
        // 网关域没有本地令牌也要继续：启动引导会带回「网关注入身份对应的应用
        // 账号」，刷新后正是靠它保住登录态（见 utils/gateway.ts）。
        if (!token.value && !onFnOSGatewayOrigin()) return
        const auth = data.auth
        if (auth?.authenticated && auth.user) {
          user.value = auth.user
          // 安全提示与主题偏好都不参与路由决策，放到后台补齐，绝不阻塞首屏。
          void loadUserPreferences()
        } else {
          clearSession()
        }
        return
      }

      // 引导请求全部失败（服务未就绪、网关不通）：按默认值继续渲染，至少
      // 页面能出来。已持有令牌时仍单独确认一次，避免网络抖动把已登录用户
      // 请到登录页。
      setBootHint('网络较慢，正在加载应用…')
      if (token.value) {
        const authRes = await withTimeout(checkAuth(), STARTUP_TIMEOUT_MS, null)
        if (authRes?.data?.code === 0 && authRes.data.data?.authenticated) {
          user.value = authRes.data.data.user
          void loadUserPreferences()
        } else {
          clearSession()
        }
      }
    })().finally(() => {
      initialized = true
    })

    return initPromise
  }

  // 登录后的补充信息：安全提示状态与用户主题偏好。失败静默，不阻塞首屏。
  async function loadUserPreferences() {
    try {
      const secRes = await getSecurityQuestions()
      if (secRes.data?.code === 0) {
        hasSecurityQuestions.value = !!secRes.data.data?.has_questions
      }
    } catch {}
    try {
      const metaRes = await getUserConfigMeta()
      if (metaRes.data?.code === 0 && Array.isArray(metaRes.data.data)) {
        const themeItem = metaRes.data.data.find((it: any) => it.key === 'theme_mode')
        if (themeItem && ['system', 'light', 'dark'].includes(themeItem.value)) {
          const themeStore = useThemeStore()
          if (themeStore.mode !== themeItem.value) {
            themeStore.setMode(themeItem.value)
          }
        }
      }
    } catch {}
  }

  function resetInit() {
    initialized = false
    initPromise = null
  }

  function setToken(newToken: string) {
    token.value = newToken
    localStorage.setItem(SK.token, newToken)
  }

  function setUser(newUser: any) {
    user.value = newUser
  }

  // 只清本地登录态。网关域上的退出不能只做这一步（服务端还能靠网关注入身份
  // 认回来），所以对外一律用 endSession()，这个函数留作它的最后一步与
  // 401 拦截器里的强制清理。
  function clearLocalSession() {
    token.value = ''
    user.value = null
    localStorage.removeItem(SK.token)
    localStorage.removeItem(DISMISS_KEY)
    // 一次性飞牛票据不能跨会话残留：换 NAS 账号登录时必须重新取票。
    sessionStorage.removeItem(SK.fnosTicket)
    sessionStorage.removeItem(SK.fnosTicketUsername)
    securityPromptDismissed.value = false
  }

  // 退出登录（用户点击）。必须先让服务端记下「这位用户主动登出了」，再清本地：
  // 网关域上服务端每个请求都能从 X-Trim-Userid 认出应用账号，只清前端的话
  // 下一个请求就把登录态认回来了，用户看到的正是「退不出去」。请求失败也要
  // 继续本地登出——宁可本地先退出、下次刷新由服务端再纠正，也不能把用户卡在
  // 已登录界面里。
  async function endSession() {
    try {
      await apiLogout()
    } catch {}
    clearLocalSession()
    sessionSource.value = 'app'
  }

  // 兼容旧调用点（401 拦截器等）：同步清本地，不等服务端。
  function logout() {
    clearLocalSession()
    sessionSource.value = 'app'
  }

  async function checkSetupRequired() {
    try {
      const setupRes = await apiCheckSetupRequired()
      if (setupRes.data?.code === 0) {
        setupRequired.value = !!setupRes.data.data?.setup_required
      }
    } catch {}
    return setupRequired.value
  }

  function dismissSecurityPrompt() {
    securityPromptDismissed.value = true
    localStorage.setItem(DISMISS_KEY, String(Date.now()))
  }

  function refreshSecurityQuestions() {
    if (!token.value) return
    getSecurityQuestions()
      .then(res => {
        if (res.data?.code === 0) {
          hasSecurityQuestions.value = !!res.data.data?.has_questions
          if (hasSecurityQuestions.value) {
            localStorage.removeItem(DISMISS_KEY)
          }
        }
      })
      .catch(() => {})
  }

  // 会话回收：网关域上如果这次登录态只是「网关注入的 NAS 身份」，会话实际归
  // NAS 所有——用户在飞牛桌面或飞牛 App 退出后，应用也必须退出（应用每次请求
  // 都会被服务端拒绝）。前端的 X-Trim-* 是网关加在同源请求上的，JS 读不到、
  // 更监听不到退出登录，所以只能周期性问服务端「NAS 那侧还认我吗」。
  //
  // 只在 followsFnOSSession 成立时才问：应用自己的会话（显式登录换来、以及
  // 直连端口）不该因为 NAS 退出而被踢下线，那正是用户要的「应用可以独立退出」。
  // 返回 true 表示确实因为 NAS 会话结束而被登出。
  async function dropIfFnOSSessionGone(): Promise<boolean> {
    if (!followsFnOSSession.value || !user.value) return false
    try {
      const res = await getBootstrap()
      const data = res.data?.code === 0 ? (res.data.data as BootstrapData) : null
      if (!data) return false
      if (data.auth?.authenticated) {
        sessionSource.value = data.session_source === 'gateway' ? 'gateway' : 'app'
        return false
      }
      clearLocalSession()
      sessionSource.value = 'app'
      return true
    } catch {
      return false
    }
  }

  return { token, user, requireLogin, allowRegister, setupRequired, siteTitle, hasSecurityQuestions, securityPromptDismissed, servicePort, fnosGatewayEntry, fnosApp, sessionSource, followsFnOSSession, isAuthenticated, isAdmin, init, resetInit, setToken, setUser, endSession, logout, dropIfFnOSSessionGone, checkSetupRequired, dismissSecurityPrompt, refreshSecurityQuestions }
})
