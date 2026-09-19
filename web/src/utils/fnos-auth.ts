// 飞牛授权：登录页 / 注册页共用。三档递进，按环境能力自动选择：
// 1) 弹窗优先（桌面浏览器）：居中弹窗打开交互式授权页（fnos-entry.html），
//    账号展示、换账号、确认、取消全部在弹窗里完成，应用主页面不跳转不刷新；
//    确认后授权页 postMessage 票据给本页并自关，本页直接登录。
// 2) 弹窗不可用（飞牛手机 App 网页视图：不支持弹窗 / 假句柄）：授权页加载后会先
//    发 ready 报到，READY_TIMEOUT_MS 内收不到即判定不可用，交回调用方兜底——
//    网关域就地签票 + 页内确认框（tryIssueFnOSTicket）。飞牛桌面入口 / 手机 App
//    的应用页面本来就跑在网关域（见 fnpack/app/ui/config），这一档就是手机端的
//    正常路径：票据在应用自己的源上就地签发，全程零跳转，因此不存在被网页视图
//    拦下导航的问题。
// 3) 网关域也签不到票（只有直连端口才可能：局域网直连、旧书签）：整页跳授权页
//    （startFnOSFullPageAuthorize），确认后带 hash 回本页，main.ts 接住票据。
// 切换飞牛账号（startFnOSAccountSwitch）：弹窗直接打开飞牛登录页换号，登录成功
// 后由飞牛登录页回跳到【同源】授权页（飞牛登录页只接受同源 redirect_uri，不一致
// 会被丢弃并回落到飞牛桌面），授权页取到新账号的票据后 postMessage 回本页。
// 跳转前先探测登录页可达性：飞牛 App 的沙箱源只代理 /app/<包名>/，/login 在该源
// 下不可达，直接跳只会得到浏览器的「无法访问页面」——不可达就返回 false，由调用
// 方就地提示用户在飞牛 App 内切换账号。
import { getVersion } from '../api/config'
import { fnosTicket } from '../api/auth'
import { SK } from './storage-keys'

// 只接受指向「本应用」授权页的网关入口：同域其它应用（彩彩助手）也会写
// fnos_gateway_url 这类键，必须按本应用前缀（/app/techfunway-reminders/）
// 过滤，否则授权跳转会被拐到别家授权页——手机/桌面端因此报「无法访问
// 页面」（入口里连端口都是别的应用拼错的）。
function isOwnGatewayEntry(url: string | null | undefined): url is string {
  if (!url) return false
  try {
    return new URL(url).pathname.startsWith(import.meta.env.BASE_URL)
  } catch {
    return false
  }
}

// 应用自己的直连端口地址**不是**网关入口：网关域与直连端口是两个不同的源，
// 把直连端口当成网关会把跳板页开在直连端口上——那边的取票请求必然 401
// （取票接口只认网关 socket），页面只显示「未获取到飞牛登录信息」，用户看到
// 的就是「飞牛登录坏了」。历史 localStorage / 服务端登记里可能残留这种地址
// （路径前缀校验挡不住它），所以必须按端口再挡一层。
function isDirectServiceEntry(url: string | null | undefined, servicePort: string): boolean {
  if (!url || !servicePort) return false
  try {
    const parsed = new URL(url)
    const port = parsed.port || (parsed.protocol === 'https:' ? '443' : '80')
    return port === servicePort
  } catch {
    return false
  }
}

// 同一主机名的另一种默认端口：飞牛网关可能在 https(443) 而不是 http(80)，
// 探测阶段两种都试一次，避免只因为协议猜错就判定「打不开授权页」。
function alternateDefaultPort(url: string): string | null {
  try {
    const parsed = new URL(url)
    if (parsed.port && parsed.port !== '80' && parsed.port !== '443') return null
    parsed.protocol = parsed.protocol === 'https:' ? 'http:' : 'https:'
    parsed.port = ''
    return parsed.toString()
  } catch {
    return null
  }
}

// 应用自身的入口地址（origin + 前端 base）。授权页必须把票据交回「打开它的那个
// 应用页面」所在地址：手机端经飞牛远程地址进来时，网关主机名下的服务端口并不
// 对外可达，按「主机名 + 端口」拼出来的回跳地址必然卡死。
export function fnOSAppBase(): string {
  return `${window.location.origin}${import.meta.env.BASE_URL}`
}

// 授权页地址：带 app_base（票据交回哪个应用页面、postMessage 目标源）与
// fnos_popup 标记（仅提示，授权页实际按 window.opener 判定模式）。
function fnOSEntryUrl(url: string, popup: boolean): string {
  const entry = new URL(url)
  entry.searchParams.set('app_base', fnOSAppBase())
  if (popup) entry.searchParams.set('fnos_popup', '1')
  else entry.searchParams.delete('fnos_popup')
  return entry.toString()
}

// 先在当前页面直接尝试签发一次性票据（与彩彩助手同款策略）：应用本身运行
// 在网关域（飞牛手机 APP 内嵌网页、桌面图标、远程域名入口）时，网关会话就
// 在本域，一次请求即可拿到票据，完全不必跳转授权页——手机端网页视图里每多
// 一次整页跳转就多一处断裂点，这正是手机端飞牛登录一直失败的根因之一。
// 直连端口上该请求必然 401，这时才回落到整页授权页。
export async function tryIssueFnOSTicket(): Promise<{ ticket: string; username: string } | null> {
  try {
    const res = await fnosTicket()
    const ticket = res.data?.data?.ticket
    if (res.data?.code === 0 && typeof ticket === 'string' && ticket) {
      const username = typeof res.data.data.fnos_username === 'string' ? res.data.data.fnos_username : ''
      return { ticket, username }
    }
  } catch { /* 直连端口：回落到整页授权页 */ }
  return null
}

const POPUP_NAME = 'fnos_auth'
const POPUP_WIDTH = 520
const POPUP_HEIGHT = 700
// 授权页 ready 报到时限：授权页与本页同源、体积极小，正常远快于此。超时未收到
// 说明该环境不支持弹窗（飞牛手机 App 网页视图）或拿到的是假句柄，立即交回
// 调用方走就地签票 / 整页流程，不让用户对着不动的窗口等。
const READY_TIMEOUT_MS = 2500

export interface FnOSAuthorizeCallbacks {
  // 弹窗里点「确认使用该账号登录」后回传：票据 + 飞牛用户名。
  onTicket: (ticket: string, username: string) => void
  // 用户在弹窗里点取消 / 直接关窗（可选，一般不报错，静默收尾）。
  onCancel?: () => void
  // 弹窗不可用（被拦截、假句柄、网页视图不支持）：由调用方兜底——网关域
  // 就地签票弹页内确认框，或在直连端口整页跳授权页。
  onUnavailable?: () => void
}

// 打开授权窗口并接管票据 / 取消 / 关闭。readyTimeoutMs > 0 时启用 ready 报到
// 看门狗（授权页是本窗第一页时用）；换账号流程第一页是飞牛登录页，不会发
// ready，传 0 关闭看门狗。返回 false 表示浏览器拦截了弹窗。
function openAuthWindow(popupURL: string, cb: FnOSAuthorizeCallbacks, readyTimeoutMs: number): boolean {
  let expectedOrigin = ''
  try {
    expectedOrigin = new URL(popupURL).origin
  } catch { /* 入口异常时交回兜底 */ }
  // 居中用屏幕可用区：多显示器 / 高 DPI 下父窗口的 screenX + outerWidth 推算会偏。
  const screen = window.screen as Screen & { availLeft?: number; availTop?: number }
  const areaLeft = typeof screen.availLeft === 'number' ? screen.availLeft : 0
  const areaTop = typeof screen.availTop === 'number' ? screen.availTop : 0
  const areaWidth = screen.availWidth || screen.width || POPUP_WIDTH
  const areaHeight = screen.availHeight || screen.height || POPUP_HEIGHT
  const left = Math.max(areaLeft, Math.round(areaLeft + (areaWidth - POPUP_WIDTH) / 2))
  const top = Math.max(areaTop, Math.round(areaTop + (areaHeight - POPUP_HEIGHT) / 2))
  const popup = window.open(
    popupURL,
    POPUP_NAME,
    `popup=yes,width=${POPUP_WIDTH},height=${POPUP_HEIGHT},left=${left},top=${top}`,
  )
  if (!popup) return false
  // 闭包（看门狗 / 关窗轮询）里要反复读这个句柄，单独存一份非空引用，
  // 避免类型收窄在函数声明内失效。
  const authWindow = popup
  let settled = false
  let ready = false
  let watchdog = 0
  let closePoll = 0
  const cleanup = () => {
    window.removeEventListener('message', onMessage)
    window.clearTimeout(watchdog)
    window.clearInterval(closePoll)
  }
  const settle = () => {
    if (!settled) {
      settled = true
      cleanup()
    }
  }
  const watchClose = () => {
    closePoll = window.setInterval(() => {
      try {
        if (authWindow.closed) {
          settle()
          cb.onCancel?.()
        }
      } catch {
        settle()
        cb.onCancel?.()
      }
    }, 500)
  }
  if (readyTimeoutMs > 0) {
    watchdog = window.setTimeout(() => {
      if (settled || ready) return
      settle()
      try { authWindow.close() } catch { /* 假句柄忽略 */ }
      cb.onUnavailable?.()
    }, readyTimeoutMs)
  }
  function onMessage(event: MessageEvent) {
    if (expectedOrigin && event.origin !== expectedOrigin) return
    const data = event.data as { type?: unknown; ticket?: unknown; username?: unknown } | null
    if (!data || typeof data !== 'object') return
    if (data.type === 'fnos_auth_ready') {
      // 真弹窗：撤掉看门狗，用户慢慢操作；随后只监测「用户直接关窗」。
      ready = true
      window.clearTimeout(watchdog)
      if (!closePoll) watchClose()
      return
    }
    if (data.type === 'fnos_auth_ticket' && typeof data.ticket === 'string' && data.ticket) {
      settle()
      // 由打开者关掉授权窗：脚本打开的窗口由 opener 关闭，比窗口自关可靠得多——
      // window.close() 在手机网页视图里常被忽略，用户会停在一个多余的窗口上，
      // 看不到应用页随后该显示的注册/绑定或控制台。
      try { authWindow.close() } catch { /* 关不掉时授权页会提示手动关闭 */ }
      cb.onTicket(data.ticket, typeof data.username === 'string' ? data.username : '')
      return
    }
    if (data.type === 'fnos_auth_cancel') {
      settle()
      cb.onCancel?.()
    }
  }
  window.addEventListener('message', onMessage)
  return true
}

// 启动飞牛授权：桌面端弹窗打开授权页并等待回传。弹窗被拦截或 ready 超时（假句柄）
// 时交回调用方兜底（网关域就地签票 / 直连端口整页跳授权页）。
export function startFnOSAuthorize(url: string, cb: FnOSAuthorizeCallbacks): void {
  // 手机端（含飞牛 App 网页视图）不走独立弹窗：那里的 window.close() 常被忽略，
  // 用户确认后会停在一个必须手动关闭的「授权成功」窗口上，看不到应用页随后该
  // 显示的注册 / 绑定页面。改为就地签票 + 应用内确认框（调用方的 onUnavailable
  // 兜底），点确认后直接在应用内进入下一步。
  if (fnOSMobileClient()) {
    cb.onUnavailable?.()
    return
  }
  const popupURL = fnOSEntryUrl(url, true)
  if (!openAuthWindow(popupURL, cb, READY_TIMEOUT_MS)) cb.onUnavailable?.()
}

// 整页跳交互式授权页（弹窗不可用且当前域签不到票时的最后手段，只有直连端口会走到）：
// 确认后带 hash 回本页，由 main.ts 接住票据继续登录。跳转前探一次可达性——入口可能
// 来自另一个网络（服务端登记是全局值），打不开时返回 false，由调用方就地提示，而不是
// 把页面送到浏览器的「无法访问页面」。
export async function startFnOSFullPageAuthorize(url: string): Promise<boolean> {
  const target = fnOSEntryUrl(url, false)
  if (await probeReachable(target)) {
    window.location.assign(target)
    return true
  }
  // 默认端口猜错协议（网关在 https 443 而不是 http 80）时再试另一种，避免
  // 只因协议不对就把用户挡在「打不开授权页」上。
  const alternative = alternateDefaultPort(url)
  if (alternative) {
    const altTarget = fnOSEntryUrl(alternative, false)
    if (await probeReachable(altTarget)) {
      window.location.assign(altTarget)
      return true
    }
  }
  return false
}

// 探测地址是否可达：飞牛 App 的沙箱源只代理 /app/<包名>/，网关登录页在该源下
// 连不上，直接 navigation 只会得到浏览器的「无法访问页面」。no-cors 只关心
// 「有没有响应」——网络层失败（拒绝连接 / DNS / 超时）才会 reject。
export async function probeReachable(url: string, timeoutMs = 4000): Promise<boolean> {
  const controller = new AbortController()
  const timer = window.setTimeout(() => controller.abort(), timeoutMs)
  try {
    await fetch(url, { mode: 'no-cors', cache: 'no-store', signal: controller.signal })
    return true
  } catch {
    return false
  } finally {
    window.clearTimeout(timer)
  }
}

// 切换飞牛账号：弹窗打开飞牛登录页换号，redirect_uri 指回【同源】授权页
// （飞牛登录页只接受同源回跳，不一致会被丢弃并回落到飞牛桌面），登录成功后
// 授权页取到新账号的票据再 postMessage 回本页，应用页全程不跳转。
// 入口按「登录页真的连得上」挑选：当前源自身（应用就在网关域时唯一一定可达）
// 优先，再退回调用方解析出来的入口——服务端登记是全局值，可能来自另一个网络
// （只在内网打开过就记下 10.x 内网地址），照搬会把远程访问的用户送到打不开的
// 地址。全部候选都不可达时返回 false，由调用方就地提示。
export async function startFnOSAccountSwitch(
  gatewayURL: string,
  servicePort: string,
  cb: FnOSAuthorizeCallbacks,
): Promise<boolean> {
  // 手机端不提供换账号入口（按钮已隐藏）；这里再兜一层，避免任何调用路径在
  // 手机端开出关不掉的窗口。返回 false 由调用方就地提示。
  if (fnOSMobileClient()) return false
  const candidates: string[] = []
  const originEntry = fnOSOriginEntry(servicePort)
  if (originEntry) candidates.push(originEntry)
  if (gatewayURL && !candidates.includes(gatewayURL)) candidates.push(gatewayURL)
  let target = ''
  for (const candidate of candidates) {
    try {
      if (await probeReachable(`${new URL(candidate).origin}/login`)) {
        target = candidate
        break
      }
    } catch { /* 入口异常，试下一个 */ }
  }
  if (!target) return false
  const authPage = fnOSEntryUrl(target, true)
  const loginURL = `${new URL(target).origin}/login?redirect_uri=${encodeURIComponent(authPage)}`
  if (!openAuthWindow(loginURL, cb, 0)) {
    // 弹窗被拦截：整页导航换号（登录成功后飞牛登录页回跳授权页，再由授权页
    // 带 hash 回本页）。此路径仅在弹窗不可用时使用。
    window.location.assign(loginURL)
  }
  return true
}

// 手机端（含飞牛手机 App 的网页视图）判定。手机端不提供「切换飞牛账号」入口：
// 该动作必须打开飞牛登录页换号，在手机端无法可靠完成（登录页在部分网络/网页视图
// 下打不开，用户会被送到浏览器错误页）。2026-09-16 按要求改为手机端隐藏该按钮、
// 提示用户在飞牛 App 内切换账号；桌面端保留。iPadOS 13+ 的 Safari 会把自己报成
// Macintosh，靠 maxTouchPoints + 粗指针兜住。
export function fnOSMobileClient(): boolean {
  const ua = navigator.userAgent || ''
  if (/Android|iPhone|iPad|iPod|Mobile|HarmonyOS|Windows Phone/i.test(ua)) return true
  return navigator.maxTouchPoints > 1 && window.matchMedia('(pointer: coarse)').matches
}

// 解析网关授权页地址。优先级：**当前源就是网关域时一律用当前源**——桌面图标、
// 飞牛手机 App、远程域名入口下，应用自己就住在网关域，当前地址是唯一一定可达的
// 入口。服务端登记（/api/version 的 fnosGatewayEntry）是全局内存值，可能来自
// 另一个网络：只在内网打开过就会记下 10.x 的内网地址，远程访问的用户照搬过去
// 必然「无法访问页面」（2026-09-16 手机端换账号踩到）。只有当前源不是网关域
// （直连端口）时才用跳板页 / 桌面 referrer 记下的入口，最后才按默认端口猜。
export function fnOSGatewayEntry(servicePort: string, serverEntry?: string): string | null {
  const fromOrigin = fnOSOriginEntry(servicePort)
  if (fromOrigin) return fromOrigin
  const stored = localStorage.getItem(SK.fnosGatewayUrl)
  if (isOwnGatewayEntry(stored) && !isDirectServiceEntry(stored, servicePort)) return stored
  if (isOwnGatewayEntry(stored)) {
    // 残留的直连端口地址：清掉，别让它继续把跳板页开在直连端口上。
    try { localStorage.removeItem(SK.fnosGatewayUrl) } catch {}
  }
  if (isOwnGatewayEntry(serverEntry) && !isDirectServiceEntry(serverEntry, servicePort)) {
    try {
      localStorage.setItem(SK.fnosGatewayUrl, serverEntry)
    } catch {}
    return serverEntry
  }
  // 与彩彩助手一致的兜底规则：同 hostname 的默认端口（80/443）即飞牛网关域。
  // 手机端首次从直连端口打开时 localStorage 与服务端登记都可能为空，按这条
  // 规则仍能拼出授权页地址，不再直接判「请先从飞牛桌面打开本应用」。
  return `${window.location.protocol}//${window.location.hostname}${import.meta.env.BASE_URL}fnos-entry.html`
}

// 当前源自身的网关入口：只在与后端服务端口不一致（= 应用就跑在网关域上，而不是
// 直连端口）时有意义。返回值同时写入 localStorage，并把入口登记到服务端，供直连
// 端口的访问者使用。
export function fnOSOriginEntry(servicePort: string): string | null {
  const onGatewayOrigin = !!servicePort && window.location.port !== servicePort
  if (!onGatewayOrigin) return null
  const fromOrigin = `${window.location.origin}${import.meta.env.BASE_URL}fnos-entry.html`
  registerGatewayEntry(fromOrigin)
  try {
    localStorage.setItem(SK.fnosGatewayUrl, fromOrigin)
  } catch {}
  return fromOrigin
}

// 应用跑在网关域时，顺手把当前网关入口登记到服务端：直连端口的访问者（没有
// localStorage、也没有 referrer）随后能从引导接口拿到真实入口，而不是只能
// 靠「默认端口」猜——网关不在默认端口时猜出来的地址必然拒绝连接。
let gatewayEntryRegistered = false
function registerGatewayEntry(entry: string): void {
  if (gatewayEntryRegistered) return
  gatewayEntryRegistered = true
  void fetch(`${import.meta.env.BASE_URL}api/auth/fnos/entry`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ url: entry }),
  }).catch(() => { gatewayEntryRegistered = false })
}

// 解析可用的网关入口，并消除「页面刚打开、/api/version 还没回来就点击」的竞态：
// 缓存里没有服务端口与入口时自行补拉一次 /api/version，避免把一次正常的点击
// 误判成「请先从飞牛桌面打开本应用」。仍未取到才返回 null。
export async function resolveFnOSGatewayEntry(cached: {
  servicePort?: string
  entry?: string
}): Promise<string | null> {
  const immediate = fnOSGatewayEntry(cached.servicePort || '', cached.entry)
  if (immediate) return immediate
  try {
    const res = await getVersion()
    if (res.data?.code !== 0) return null
    const servicePort = res.data.data?.servicePort || ''
    const entry = res.data.data?.fnosGatewayEntry
    const serverEntry = isOwnGatewayEntry(entry) ? entry : ''
    if (serverEntry) {
      try {
        localStorage.setItem(SK.fnosGatewayUrl, serverEntry)
      } catch {}
    }
    return fnOSGatewayEntry(servicePort, serverEntry)
  } catch {
    return null
  }
}
