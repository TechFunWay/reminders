import { SK } from './storage-keys'

// 本标签页的客户端标识。
//
// 用途只有一个：服务端广播「数据变了」时跳过发起变更的那个标签页——它已经在本地
// 更新过界面，再收一次自己的回声只会白白多刷一遍。
//
// 用 sessionStorage 而不是 localStorage：同一台设备开两个标签页就是两个客户端，
// 在 A 标签页改的提醒，B 标签页也应该自动刷新（这正是用户要的「换个设备/换个窗口
// 都跟着变」）。刷新保留同一个 id，所以刷新后不会把自己当成别人。
let cached = ''

export function clientID(): string {
  if (cached) return cached
  try {
    const existing = sessionStorage.getItem(SK.clientId)
    if (existing) {
      cached = existing
      return cached
    }
    const generated = typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function'
      ? crypto.randomUUID()
      : `c-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
    sessionStorage.setItem(SK.clientId, generated)
    cached = generated
  } catch {
    // 隐私模式下 sessionStorage 可能不可用。退化成「本次页面加载内稳定的随机
    // id」（缓存仍在，所以请求头与 SSE 参数一致），最坏只是刷新后换一个 id，
    // 影响仅限于变更广播不再排除发起者，功能不受影响。
    cached = `c-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 10)}`
  }
  return cached
}
