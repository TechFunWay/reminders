import request from './request'

export function getUsers(page: number, pageSize: number, search?: string) {
  return request.get('/api/users', { params: { page, pageSize, search } })
}

export function updateUser(id: number, data: Record<string, any>) {
  return request.put(`/api/users/${id}`, data)
}

export function deleteUser(id: number) {
  return request.delete(`/api/users/${id}`)
}

export function toggleUserStatus(id: number) {
  return request.put(`/api/users/${id}/status`)
}

export function resetUserPassword(id: number, newPassword: string) {
  return request.put(`/api/users/${id}/password`, { new_password: newPassword })
}
