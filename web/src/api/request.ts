import axios from 'axios'
import { useAuthStore } from '../stores/auth'
import router from '../router'
import { SK } from '../utils/storage-keys'
import { onFnOSGatewayOrigin } from '../utils/gateway'
import { clientID } from '../utils/client-id'

const request = axios.create({
  baseURL: import.meta.env.BASE_URL,
  timeout: 30000,
})

request.interceptors.request.use((config) => {
	if (import.meta.env.BASE_URL !== '/' && config.url?.startsWith('/')) {
		config.url = config.url.slice(1)
	}
  const authStore = useAuthStore()
  // 飞牛网关域上不带应用自己的 Authorization：接入层会把它当成自己的会话 token，
  // 认不出就直接回 "invalid token"，请求到不了应用（详见 utils/gateway.ts）。
  // 那里的登录态由服务端用网关注入的 X-Trim-* 身份解析；直连端口照旧带 JWT。
  if (authStore.token && !onFnOSGatewayOrigin()) {
    config.headers.Authorization = `Bearer ${authStore.token}`
  }
  // 飞牛授权登录的直连端口凭证：由网关跳板页签发，登录/绑定成功后由后端消费。
  const fnosTicket = sessionStorage.getItem(SK.fnosTicket)
  if (fnosTicket) {
    config.headers['X-FnOS-Ticket'] = fnosTicket
  }
  // 本标签页标识：服务端据此在广播「数据变了」时跳过发起者自己（见 utils/client-id.ts）。
  const id = clientID()
  if (id) config.headers['X-Client-Id'] = id
  return config
})

// 401 是否属于「预期分支，不要当成登录失效」。axios v1 的 config.headers 是
// AxiosHeaders 实例，直接按原大小写取属性不可靠，所以先走它的 get()（大小写不
// 敏感），再退回普通对象取值。取不到就等于没标记——失败方向是「仍然登出」，
// 与旧行为一致，不会悄悄放过真正的登录失效。
function skipsAuthRedirect(config: any): boolean {
  const headers = config?.headers
  if (!headers) return false
  if (typeof headers.get === 'function') {
    return !!headers.get('X-Skip-Auth-Redirect')
  }
  return !!(headers['X-Skip-Auth-Redirect'] || headers['x-skip-auth-redirect'])
}

request.interceptors.response.use(
  (response) => {
    return response
  },
  (error) => {
    // 探测类请求（飞牛就地签票）与飞牛授权登录/绑定请求的 401 都是预期分支：
    // 前者在直连端口必然 401，后者是「这张票据不认」——都不该当成登录失效被
    // 强制拽回登录页，否则用户看到的就是「点了飞牛登录又回到登录页」。
    // 401 是服务端已经拒绝了这个会话：本地直接清干净即可，不必再调
    // /api/auth/logout（那次调用同样会 401）。网关域上的「主动登出」走
    // authStore.endSession()，会先落服务端标记再清本地。
    if (error.response?.status === 401 && !skipsAuthRedirect(error.config)) {
      const authStore = useAuthStore()
      authStore.logout()
      router.push('/login')
    }
    return Promise.reject(error)
  }
)

export default request
