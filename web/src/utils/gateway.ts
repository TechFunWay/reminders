// 当前页面是不是由飞牛统一网关的 socket 服务出来的。
//
// 服务端在返回入口 HTML 时把 __FNOS_GATEWAY__ 换成 true/false（见
// server/server/server.go）：只有服务端知道这次文档请求走的是网关 socket 还是
// 应用自己的直连端口，而前端必须据此决定「带不带自己的 JWT」。
//
// 为什么关键：飞牛统一网关的接入层会把请求里的 Authorization 当成**它自己的**
// 会话 token。应用自己的 `Bearer <jwt>` 它认不出来，于是直接返回 200 纯文本
// "invalid token"，请求根本到不了应用——表现就是「飞牛登录后一刷新就回登录页」
// （启动引导拿不到自己的登录态，前端按未登录处理）。
// 所以网关域上一律不带 Authorization，登录态改由服务端用网关注入的
// X-Trim-Userid/Username 解析成「已绑定的应用账号」；直连端口照旧带 JWT。
export function onFnOSGatewayOrigin(): boolean {
  return (window as { __FNOS_GATEWAY__?: boolean }).__FNOS_GATEWAY__ === true
}
