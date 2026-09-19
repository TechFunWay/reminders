import { createApp } from 'vue'
import { createPinia } from 'pinia'
import router from './router'
import App from './App.vue'
import './style.css'
import { dismissBootSplash } from './utils/boot'
import { SK } from './utils/storage-keys'

// 飞牛授权走「网关跳板 + 一次性票据」：跳板页把 fnos_ticket 带回应用
// 直连端口，这里先接住；后续 /api/auth/fnos/login、/bind 请求由 request
// 拦截器附带 X-FnOS-Ticket。存储键统一带应用前缀（见 utils/storage-keys），
// 与同域其它应用（彩彩助手）互不覆盖。
function captureFnOSTicket() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const ticket = params.get('fnos_ticket')
  if (!ticket) return
  sessionStorage.setItem(SK.fnosTicket, ticket)
  const fnosUsername = params.get('fnos_username')
  if (fnosUsername) sessionStorage.setItem(SK.fnosTicketUsername, fnosUsername)
  params.delete('fnos_ticket')
  params.delete('fnos_username')
  params.delete('fnos_gateway')
  const rest = params.toString()
  history.replaceState(null, '', window.location.pathname + window.location.search + (rest ? `#${rest}` : ''))
}

// 跳板页取票/跳转失败时把错误标记带回直连端口，登录页据此就地提示，
// 避免用户停在空白跳板页不知所措。
function captureFnOSEntryError() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const error = params.get('fnos_error')
  if (!error) return
  sessionStorage.setItem(SK.fnosEntryError, error)
  params.delete('fnos_error')
  params.delete('fnos_gateway')
  const rest = params.toString()
  history.replaceState(null, '', window.location.pathname + window.location.search + (rest ? `#${rest}` : ''))
}

// 跳板页回带网关入口：从桌面图标打开时 referrer 往往为空，这是最可靠的
// 网关地址来源，登录页据此才能重新发起飞牛授权登录。只接受指向本应用
// 跳板页的入口，同域其它应用的入口一律丢弃。
function rememberFnOSGatewayFromHash() {
  const hash = window.location.hash.replace(/^#/, '')
  if (!hash) return
  const params = new URLSearchParams(hash)
  const gateway = params.get('fnos_gateway')
  if (!gateway) return
  try {
    if (new URL(gateway).pathname.startsWith(import.meta.env.BASE_URL)) {
      localStorage.setItem(SK.fnosGatewayUrl, gateway)
    }
  } catch {}
}

// 从飞牛桌面跳转过来时记住网关入口，登录页「使用飞牛 NAS 登录」按钮据此
// 打开跳板页。
function rememberFnOSGateway() {
  if (!document.referrer) return
  try {
    const desktop = new URL(document.referrer)
    if (desktop.protocol !== 'http:' && desktop.protocol !== 'https:') return
    if (desktop.hostname !== window.location.hostname) return
    if (desktop.origin === window.location.origin) return
    localStorage.setItem(SK.fnosGatewayUrl, `${desktop.origin}${import.meta.env.BASE_URL}fnos-entry.html`)
  } catch {}
}

// 顺序要紧：会话信息与错误标记的清理都会顺手把 fnos_gateway 从 hash 里摘掉，
// 所以必须先把网关入口记下来，否则整页跳板回来这一趟记不住入口，下一次点
// 「使用飞牛 NAS 登录」还得重新去找（手机端首次打开尤其明显）。
rememberFnOSGatewayFromHash()
captureFnOSTicket()
captureFnOSEntryError()
rememberFnOSGateway()

const app = createApp(App)
app.use(createPinia())
app.use(router)
app.mount('#app')

// 首个路由（含全局守卫里的启动引导请求）落定后才收起首屏骨架：在那之前
// router-view 还是空的，过早移除等于把白屏还回去。
void router.isReady().then(dismissBootSplash)
