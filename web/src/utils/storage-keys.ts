// 本地存储键统一加应用前缀。本应用与彩彩助手等应用都会跑在飞牛网关域下，
// 同域即共享同一 localStorage/sessionStorage：裸键名会互相覆盖——彩彩助手
// 每次启动都会重写 fnos_gateway_url 为它自己的跳板页地址（且按默认端口拼
// 写），本应用读到后授权跳转就会拐到别家跳板页；它写入的 token 也会顶掉
// 本应用的登录态。加前缀后各写各的键，互不干扰。旧裸键不再读取也不再删
// 除（同域上无法分辨归属，删除可能误伤其它应用的会话）。
const P = 'reminders.'

export const SK = {
  token: `${P}token`,
  fnosGatewayUrl: `${P}fnos_gateway_url`,
  fnosPopupOrigin: `${P}fnos_popup_origin`,
  fnosTicket: `${P}fnos_ticket`,
  fnosTicketUsername: `${P}fnos_ticket_username`,
  fnosEntryError: `${P}fnos_entry_error`,
  // 安全提示的「稍后再说」时间戳：也必须是带前缀的键——网关域下与兄弟应用
  // 共享 localStorage，裸键会互相覆盖。
  securityPromptDismissed: `${P}security_prompt_dismissed_at`,
  // 本标签页的客户端标识（sessionStorage，一个标签页一个）：随请求与 SSE 连接
  // 一起发给服务端，服务端广播数据变更时据此跳过发起者自己。同样必须带前缀。
  clientId: `${P}client_id`,
} as const
