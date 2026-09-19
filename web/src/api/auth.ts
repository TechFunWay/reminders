import request from './request'

export function login(username: string, password: string) {
  return request.post('/api/auth/login', {
    username,
    password,
  })
}

export function register(username: string, password: string) {
  return request.post('/api/auth/register', {
    username,
    password,
  })
}

// 飞牛一键登录：处理的是「授权拿到的票据」，401 是「这张票据不认 / 过期」，
// 属预期分支——就地显示错误，不能触发全局的「401 即登出并跳登录页」，否则
// 用户看到的就是「点了飞牛登录又回到登录页」，还丢了错误原因。
export function fnosLogin() {
  return request.post('/api/auth/fnos/login', null, {
    headers: { 'X-Skip-Auth-Redirect': '1' },
  })
}

// 一次性登录票据：仅网关连接可签发。应用页以此探测自己是否运行在网关域
// （飞牛手机 APP / 桌面图标 / 远程入口），成功即可就地完成授权，无需跳板页。
// 直连端口上该请求必然 401，属预期分支，要跳过全局的「401 即登出」拦截。
export function fnosTicket() {
  return request.post('/api/auth/fnos/ticket', null, {
    headers: { 'X-Skip-Auth-Redirect': '1' },
  })
}

// 飞牛绑定 / 创建：同 fnosLogin，401 只代表票据问题，就地提示即可。
export function bindFnOSAccount(mode: 'register' | 'bind', username: string, password: string) {
  return request.post('/api/auth/fnos/bind', { mode, username, password }, {
    headers: { 'X-Skip-Auth-Redirect': '1' },
  })
}

export function checkAuth() {
  return request.get('/api/auth/check')
}

// 退出登录：服务端要落一条「别再自动认人」的标记，网关域上光清本地是退不掉的
// （网关注入的 NAS 身份会让下一个请求把登录态认回来）。它幂等，且 401 属预期
// 分支——会话本来就失效时也必须能退干净，所以跳过全局的「401 即跳登录页」。
export function logout() {
  return request.post('/api/auth/logout', null, {
    headers: { 'X-Skip-Auth-Redirect': '1' },
  })
}

export function getCurrentUser() {
  return request.get('/api/auth/me')
}

export function checkSetupRequired() {
  return request.get('/api/auth/setup-required')
}

export function changePassword(oldPassword: string, newPassword: string) {
  return request.put('/api/auth/password', {
    old_password: oldPassword,
    new_password: newPassword,
  })
}

export function regenerateAPIKey() {
  return request.post('/api/auth/apikey')
}
