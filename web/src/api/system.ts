import request from './request'

// 系统级通用接口：赞赏支持计数。
// 前端永远只请求本地后端，由后端转发到统计端点（避免跨域与混合内容）。

// amount 赞赏金额（元），随 donate_support 事件上报到统计端点，
// 由接收端记录到 app_support_log.amount；缺省 0 表示未填写。
export function donateSupport(amount?: number) {
  return request.post<{ code: number; message: string; data: { ok: boolean } }>('/api/donate/support', {
    amount: amount && amount > 0 ? amount : 0,
  })
}
