import request from './request'

// 首屏引导信息：一次请求拿到渲染第一帧所需的全部内容。
//
// 此前是「公开配置 + 初始化状态（并行）+ 令牌校验」两到三次往返，弱网下
// 每多一次往返就是多等一截白屏；合并后首屏只需一个 RTT，且飞牛部署下
// 顺带带回服务端口与网关入口，登录页不必再单独查一次版本信息。
export interface BootstrapAuth {
  authenticated: boolean
  require_login?: string
  user: { id: number; username: string; role: string } | null
}

export interface BootstrapData {
  setup_required: boolean
  configs: Record<string, string>
  auth: BootstrapAuth
  // 后端是否真以飞牛应用模式（-fnos-app）运行。登录/注册页据此显隐
  // 「使用飞牛 NAS 登录」：同一套飞牛版前端产物也会被裸二进制 / Docker
  // 跑起来，非飞牛部署必须藏掉按钮，点了只会得到「请先从飞牛桌面打开
  // 本应用」的死路。
  fnos_app?: boolean
  service_port?: string
  fnos_gateway_entry?: string
  // 本次登录态的来源，仅飞牛网关域上返回：
  //   gateway —— 只靠网关注入的 NAS 身份成立（用户没在本应用里显式登录过），
  //              会话归 NAS 所有，NAS 那侧退出后应用要跟着退出；
  //   app     —— 应用自己的会话（显式登录换来），可以独立退出，不跟随 NAS。
  session_source?: 'app' | 'gateway'
}

export function getBootstrap() {
  return request.get('/api/bootstrap', { timeout: 15000 })
}
