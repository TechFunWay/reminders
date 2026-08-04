import request from './request'

export interface AuditQuery {
  page?: number
  pageSize?: number
  action?: string
  username?: string
  user_id?: number
}

export function getAuditLogs(params: AuditQuery) {
  return request.get('/api/audit-logs', { params })
}

// exportUrl builds the CSV export URL (the request needs the auth header, so
// callers fetch it as a blob rather than navigating directly).
export function exportAuditLogs(params: AuditQuery) {
  return request.get('/api/audit-logs/export', { params, responseType: 'blob' })
}
