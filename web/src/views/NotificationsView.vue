<template>
  <div class="mx-auto max-w-4xl">
    <div class="mb-7 flex items-end justify-between">
      <div>
        <p class="mb-2 text-xs font-extrabold uppercase tracking-[.18em] text-brand-500">消息与动态</p>
        <h1 class="font-display text-3xl font-extrabold tracking-tight">通知中心</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ unread }} 条未读通知</p>
      </div>
      <button v-if="unread" class="rounded-xl border border-border bg-surface px-4 py-2 text-xs font-bold text-foreground transition hover:bg-muted" @click="readAll">全部标为已读</button>
    </div>

    <div v-if="loading" class="space-y-3">
      <div v-for="n in 4" :key="n" class="h-24 animate-pulse rounded-2xl bg-surface"></div>
    </div>

    <div v-else-if="notifications.length" class="overflow-hidden rounded-[24px] border border-border/70 bg-surface/85 shadow-card backdrop-blur-xl">
      <button
        v-for="item in notifications"
        :key="item.id"
        class="notification-row group"
        :class="{ unread: !item.read_at }"
        @click="read(item)"
      >
        <span class="notification-icon" :class="item.type === 'channel_failed' ? 'failure' : 'due'">
          <svg v-if="item.type === 'channel_failed'" viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="M12 9v4m0 4h.01M10.3 3.8 2.5 17.2A2 2 0 0 0 4.2 20h15.6a2 2 0 0 0 1.7-2.8L13.7 3.8a2 2 0 0 0-3.4 0Z" stroke-width="1.8" stroke-linecap="round"/></svg>
          <svg v-else viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" stroke-width="1.8" stroke-linecap="round"/></svg>
        </span>
        <span class="min-w-0 flex-1 text-left">
          <span class="flex items-center gap-2">
            <strong class="truncate text-sm">{{ item.title }}</strong>
            <span v-if="!item.read_at" class="h-2 w-2 shrink-0 rounded-full bg-brand-500"></span>
          </span>
          <span class="mt-1 block text-xs leading-5 text-muted-foreground">{{ item.body }}</span>
          <span class="mt-2 block text-[11px] font-medium text-muted-foreground/70">{{ formatTime(item.created_at) }}</span>
        </span>
        <svg class="h-4 w-4 text-muted-foreground/40 opacity-0 transition group-hover:opacity-100" viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="m9 5 7 7-7 7" stroke-width="2" stroke-linecap="round"/></svg>
      </button>
    </div>

    <div v-else class="flex min-h-[420px] flex-col items-center justify-center rounded-[28px] border border-dashed border-border bg-surface/35 text-center">
      <span class="flex h-20 w-20 items-center justify-center rounded-[24px] bg-brand-500/10 text-brand-500">
        <svg viewBox="0 0 24 24" class="h-9 w-9" fill="none" stroke="currentColor"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" stroke-width="1.6" stroke-linecap="round"/></svg>
      </span>
      <h2 class="mt-5 text-lg font-extrabold">这里很安静</h2>
      <p class="mt-1 text-sm text-muted-foreground">到期提醒和渠道状态会出现在这里。</p>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { getNotifications, markAllNotificationsRead, markNotificationRead } from '../api/reminder'
import type { ReminderRealtimeEvent, RealtimeNotification } from '../services/reminderRealtime'

type NotificationItem = RealtimeNotification

const notifications = ref<NotificationItem[]>([])
const loading = ref(true)
const unread = computed(() => notifications.value.filter(x => !x.read_at).length)

async function load() {
  loading.value = true
  try {
    const res = await getNotifications()
    notifications.value = res.data.data || []
  } finally {
    loading.value = false
  }
}
async function read(item: NotificationItem) {
  if (!item.read_at) {
    await markNotificationRead(item.id)
    item.read_at = new Date().toISOString()
    window.dispatchEvent(new CustomEvent('reminder-data-changed'))
  }
}
async function readAll() {
  await markAllNotificationsRead()
  const now = new Date().toISOString()
  notifications.value.forEach(x => { if (!x.read_at) x.read_at = now })
  window.dispatchEvent(new CustomEvent('reminder-data-changed'))
}
function formatTime(value: string) {
  const d = new Date(value)
  const diff = Date.now() - d.getTime()
  if (diff < 60_000) return '刚刚'
  if (diff < 3_600_000) return `${Math.floor(diff / 60_000)} 分钟前`
  if (diff < 86_400_000) return `${Math.floor(diff / 3_600_000)} 小时前`
  return new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', hour: '2-digit', minute: '2-digit' }).format(d)
}
function receiveRealtime(event: Event) {
  const notification = (event as CustomEvent<ReminderRealtimeEvent>).detail?.notification
  if (!notification || notifications.value.some(item => item.id === notification.id)) return
  notifications.value.unshift(notification)
}
function refreshAfterResume() { void load() }

onMounted(() => {
  load()
  window.addEventListener('reminder-realtime', receiveRealtime)
  window.addEventListener('reminder-realtime-resume', refreshAfterResume)
})
onBeforeUnmount(() => {
  window.removeEventListener('reminder-realtime', receiveRealtime)
  window.removeEventListener('reminder-realtime-resume', refreshAfterResume)
})
</script>

<style scoped>
.notification-row { @apply relative flex w-full items-center gap-4 border-b border-border/60 px-5 py-5 transition last:border-b-0 hover:bg-muted/50; }
.notification-row.unread { @apply bg-brand-500/[.035]; }
.notification-row.unread::before { content: ''; position: absolute; inset: 12px auto 12px 0; width: 3px; border-radius: 0 4px 4px 0; background: rgb(var(--color-brand-500)); }
.notification-icon { @apply flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl; }
.notification-icon.due { @apply bg-blue-500/10 text-blue-500; }
.notification-icon.failure { @apply bg-rose-500/10 text-rose-500; }
</style>
