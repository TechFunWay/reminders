import request from './request'

export type ReminderChannel = 'inapp' | 'email' | 'sms' | 'feishu' | 'qq'

export interface ReminderList {
  id: number
  name: string
  color: string
  icon: string
  position: number
  is_default: boolean
  open_count: number
}

export interface ReminderItem {
  id: number
  list_id: number
  list_name: string
  title: string
  notes: string
  priority: number
  due_at: string | null
  all_day: boolean
  repeat_rule: string
  completed_at: string | null
  snoozed_until: string | null
  version: number
  channels: ReminderChannel[]
  created_at: string
  updated_at: string
}

export interface SaveReminderInput {
  title: string
  notes?: string
  list_id?: number
  priority?: number
  due_at?: string | null
  all_day?: boolean
  repeat_rule?: string
  channels?: ReminderChannel[]
  version?: number
}

export interface ChannelStatus {
  channel: ReminderChannel
  label: string
  configured: boolean
  bound: boolean
  status: string
  target_masked?: string
  bot_link?: string
  description: string
}

export const getSummary = () => request.get('/api/reminder/summary')
export const getLists = () => request.get('/api/reminder/lists')
export const createList = (data: { name: string; color?: string; icon?: string }) =>
  request.post('/api/reminder/lists', data)
export const updateList = (id: number, data: Partial<ReminderList>) =>
  request.patch(`/api/reminder/lists/${id}`, data)
export const deleteList = (id: number) => request.delete(`/api/reminder/lists/${id}`)

export const getReminders = (params: { view?: string; list_id?: number; q?: string }) =>
  request.get('/api/reminder/items', { params })
export const getReminder = (id: number) => request.get(`/api/reminder/items/${id}`)
export const createReminder = (data: SaveReminderInput) => request.post('/api/reminder/items', data)
export const updateReminder = (id: number, data: SaveReminderInput) =>
  request.put(`/api/reminder/items/${id}`, data)
export const deleteReminder = (id: number) => request.delete(`/api/reminder/items/${id}`)
export const completeReminder = (id: number) => request.post(`/api/reminder/items/${id}/complete`)
export const restoreReminder = (id: number) => request.post(`/api/reminder/items/${id}/restore`)
export const snoozeReminder = (id: number, until: string) =>
  request.post(`/api/reminder/items/${id}/snooze`, { until })

export const getNotifications = () => request.get('/api/reminder/notifications')
export const getUnreadCount = () => request.get('/api/reminder/notifications/unread-count')
export const markNotificationRead = (id: number) => request.post(`/api/reminder/notifications/${id}/read`)
export const markAllNotificationsRead = () => request.post('/api/reminder/notifications/read-all')

export const getChannelStatuses = () => request.get('/api/reminder/channels')
export const bindChannel = (channel: ReminderChannel, target: string) =>
  request.put(`/api/reminder/channels/${channel}`, { target })
export const toggleChannel = (channel: ReminderChannel, enabled: boolean) =>
  request.patch(`/api/reminder/channels/${channel}`, { enabled })
export const unbindChannel = (channel: ReminderChannel) =>
  request.delete(`/api/reminder/channels/${channel}`)
export const testChannel = (channel: ReminderChannel) =>
  request.post(`/api/reminder/channels/${channel}/test`)
export const getProviderStatuses = () => request.get('/api/reminder/admin/providers')
export const getNotificationBrand = () => request.get('/api/reminder/admin/notification-brand')
export const saveNotificationBrand = (name: string) => request.put('/api/reminder/admin/notification-brand', { name })
export const saveFeishuProvider = (data: { app_id: string; app_secret: string }) =>
  request.put('/api/reminder/admin/providers/feishu', data)
export const saveProvider = (provider: 'email' | 'sms' | 'qq', data: Record<string, string>) =>
  request.put(`/api/reminder/admin/providers/${provider}`, data)
export const createQQBindCode = () => request.post('/api/reminder/channels/qq/bind-code')
