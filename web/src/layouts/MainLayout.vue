<template>
  <div class="min-h-screen text-foreground">
    <div class="ambient" aria-hidden="true">
      <span class="ambient-orb ambient-orb-one"></span>
      <span class="ambient-orb ambient-orb-two"></span>
    </div>

    <aside class="sidebar" :class="{ 'sidebar-open': mobileOpen }">
      <div class="flex h-full flex-col">
        <div class="flex items-center justify-between px-5 pb-5 pt-6">
          <RouterLink to="/admin" class="group flex items-center gap-3" @click="mobileOpen = false">
            <span class="logo-mark">
              <svg viewBox="0 0 24 24" class="h-6 w-6" fill="none" stroke="currentColor">
                <path d="m7 12 3 3 7-7" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/>
              </svg>
            </span>
            <span>
              <span class="flex items-center gap-2">
                <span class="font-display text-lg font-extrabold tracking-tight">{{ authStore.siteTitle || '提醒事项' }}</span>
                <span v-if="appVersion" class="version-badge" :title="appBuildTime ? `构建时间：${appBuildTime}` : ''">{{ appVersion }}</span>
              </span>
              <span class="block text-[11px] font-medium text-muted-foreground">把重要的事，放心交给我</span>
            </span>
          </RouterLink>
          <button class="icon-button mobile-only" aria-label="关闭菜单" @click="mobileOpen = false">
            <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="m6 6 12 12M18 6 6 18" stroke-width="2" stroke-linecap="round"/></svg>
          </button>
        </div>

        <button class="new-reminder-button mx-4" @click="openQuickAdd">
          <span class="flex h-8 w-8 items-center justify-center rounded-full bg-white/20">
            <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="M12 5v14M5 12h14" stroke-width="2.2" stroke-linecap="round"/></svg>
          </span>
          新建提醒
          <kbd class="ml-auto hidden rounded-md bg-black/10 px-1.5 py-0.5 text-[10px] sm:inline">N</kbd>
        </button>

        <nav class="mt-6 flex-1 overflow-y-auto px-3 pb-5">
          <p class="nav-caption">智能清单</p>
          <RouterLink
            v-for="item in smartNav"
            :key="item.to"
            :to="item.to"
            class="nav-row"
            :class="{ 'nav-row-active': isActive(item.to) }"
            @click="mobileOpen = false"
          >
            <span class="nav-symbol" :class="item.tone" v-html="item.icon"></span>
            <span class="flex-1">{{ item.label }}</span>
            <span v-if="summary[item.key]" class="nav-count">{{ summary[item.key] }}</span>
          </RouterLink>

          <div class="mt-7 flex items-center justify-between px-3">
            <p class="nav-caption !px-0 !pb-0">我的清单</p>
            <button class="tiny-add" aria-label="新建清单" @click="creatingList = true">
              <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M12 5v14M5 12h14" stroke-width="2" stroke-linecap="round"/></svg>
            </button>
          </div>

          <form v-if="creatingList" class="mx-2 mt-3 rounded-xl border border-brand-500/20 bg-brand-500/5 p-2" @submit.prevent="submitList">
            <input ref="listInput" v-model="newListName" class="w-full bg-transparent px-2 py-1 text-sm outline-none" maxlength="40" placeholder="清单名称" @keydown.esc="creatingList = false" />
            <div class="mt-2 flex justify-end gap-1">
              <button type="button" class="rounded-lg px-2 py-1 text-xs text-muted-foreground hover:bg-muted" @click="creatingList = false">取消</button>
              <button class="rounded-lg bg-brand-500 px-2 py-1 text-xs font-semibold text-white">创建</button>
            </div>
          </form>

          <RouterLink
            to="/admin/all"
            class="nav-row"
            :class="{ 'nav-row-active': isActive('/admin/all') }"
            @click="mobileOpen = false"
          >
            <span class="h-2.5 w-2.5 rounded-full bg-violet-500 ring-4 ring-violet-500/10"></span>
            <span class="flex-1">全部</span>
            <span v-if="summary.all" class="nav-count">{{ summary.all }}</span>
          </RouterLink>

          <RouterLink
            v-for="list in lists"
            :key="list.id"
            :to="`/admin/list/${list.id}`"
            class="nav-row"
            :class="{ 'nav-row-active': route.path === `/admin/list/${list.id}` }"
            @click="mobileOpen = false"
          >
            <span class="h-2.5 w-2.5 rounded-full ring-4 ring-current/10" :style="{ color: listColor(list.color), background: listColor(list.color) }"></span>
            <span class="flex-1 truncate">{{ list.name }}</span>
            <span v-if="list.open_count" class="nav-count">{{ list.open_count }}</span>
          </RouterLink>
        </nav>

        <div class="border-t border-border/70 p-3">
          <RouterLink to="/admin/channels" class="nav-row" :class="{ 'nav-row-active': route.path === '/admin/channels' }">
            <span class="nav-symbol bg-violet-500/12 text-violet-500">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M8 12h8m-8 4h5m6-11H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4l3 3 3-3h4a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2Z" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </span>
            <span class="flex-1">通知方式</span>
          </RouterLink>
          <RouterLink to="/admin/settings" class="nav-row" :class="{ 'nav-row-active': route.path === '/admin/settings' }">
            <span class="nav-symbol bg-slate-500/10 text-slate-500">
              <svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M12 15.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7Z" stroke-width="1.8"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21h-4v-.09A1.7 1.7 0 0 0 8.6 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3v-4h.09A1.7 1.7 0 0 0 4.6 8.6a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.09A1.7 1.7 0 0 0 15.4 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.13.38.35.72.65 1 .3.28.7.42 1.1.4H21v4h-.09A1.7 1.7 0 0 0 19.4 15Z" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </span>
            <span class="flex-1">偏好设置</span>
          </RouterLink>
          <template v-if="authStore.isAdmin">
            <p class="nav-caption mt-3">系统管理</p>
            <RouterLink v-for="item in adminNav" :key="item.to" :to="item.to" class="nav-row" :class="{ 'nav-row-active': isActive(item.to) }">
              <span class="nav-symbol" :class="item.tone" v-html="item.icon"></span>
              <span class="flex-1">{{ item.label }}</span>
            </RouterLink>
          </template>
        </div>
      </div>
    </aside>

    <div v-if="mobileOpen" class="fixed inset-0 z-40 bg-slate-950/30 backdrop-blur-sm lg:hidden" @click="mobileOpen = false"></div>

    <div class="min-h-screen lg:pl-[292px]">
      <header class="topbar">
        <button class="icon-button mobile-only" aria-label="打开菜单" @click="mobileOpen = true">
          <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="M4 7h16M4 12h16M4 17h16" stroke-width="2" stroke-linecap="round"/></svg>
        </button>
        <div class="hidden min-w-0 sm:block">
          <p class="text-[11px] font-bold uppercase tracking-[0.18em] text-muted-foreground">{{ dayLabel }}</p>
          <h2 class="truncate text-base font-bold">{{ greeting }}，{{ authStore.user?.username }}</h2>
        </div>
        <div class="ml-auto flex items-center gap-2">
          <RouterLink to="/admin/notifications" class="icon-button relative" aria-label="通知中心">
            <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
            <span v-if="summary.unread" class="absolute right-1 top-1 h-2.5 w-2.5 rounded-full border-2 border-surface bg-red-500"></span>
          </RouterLink>
          <UiThemeToggle />
          <div class="mx-1 h-7 w-px bg-border"></div>
          <RouterLink to="/admin/profile" class="flex items-center gap-2 rounded-xl p-1.5 pr-2 hover:bg-muted">
            <span class="avatar">{{ userInitial }}</span>
            <span class="hidden text-sm font-semibold md:block">{{ authStore.user?.username }}</span>
          </RouterLink>
          <button class="icon-button" aria-label="退出登录" @click="logout">
            <svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="m15 17 5-5-5-5M20 12H9m2 8H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2h6" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
        </div>
      </header>

      <main class="px-4 pb-28 pt-4 sm:px-6 sm:pt-6 lg:px-8 lg:pb-10 xl:px-10">
        <RouterView v-slot="{ Component }">
          <Transition name="page" mode="out-in">
            <component :is="Component" />
          </Transition>
        </RouterView>
      </main>
    </div>

    <nav class="mobile-tabbar">
      <RouterLink to="/admin" class="mobile-tab" :class="{ active: route.path === '/admin' }"><span v-html="smartNav[0].icon"></span><small>今天</small></RouterLink>
      <RouterLink to="/admin/planned" class="mobile-tab" :class="{ active: route.path === '/admin/planned' }"><span v-html="smartNav[1].icon"></span><small>计划</small></RouterLink>
      <button class="mobile-add" aria-label="新建提醒" @click="openQuickAdd"><svg viewBox="0 0 24 24" class="h-7 w-7" fill="none" stroke="currentColor"><path d="M12 5v14M5 12h14" stroke-width="2.2" stroke-linecap="round"/></svg></button>
      <RouterLink to="/admin/notifications" class="mobile-tab" :class="{ active: route.path === '/admin/notifications' }"><span><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" stroke-width="1.8" stroke-linecap="round"/></svg></span><small>通知</small></RouterLink>
      <RouterLink to="/admin/channels" class="mobile-tab" :class="{ active: route.path === '/admin/channels' }"><span><svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M8 12h8m-8 4h5m6-11H5a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h4l3 3 3-3h4a2 2 0 0 0 2-2V7a2 2 0 0 0-2-2Z" stroke-width="1.8"/></svg></span><small>方式</small></RouterLink>
    </nav>

    <Toast :message="realtimeToast" type="success" :duration="5000" />
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { RouterLink, RouterView, useRoute, useRouter } from 'vue-router'
import Toast from '../components/Toast.vue'
import UiThemeToggle from '../components/ui/ThemeToggle.vue'
import { useAuthStore } from '../stores/auth'
import { createList, getLists, getSummary, type ReminderList } from '../api/reminder'
import { getVersion } from '../api/config'
import { connectReminderEvents, type ReminderRealtimeEvent } from '../services/reminderRealtime'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const mobileOpen = ref(false)
const lists = ref<ReminderList[]>([])
const summary = ref<Record<string, number>>({})
const creatingList = ref(false)
const newListName = ref('')
const listInput = ref<HTMLInputElement | null>(null)
const realtimeToast = ref('')
const appVersion = ref('')
const appBuildTime = ref('')

const icons = {
  today: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><rect x="4" y="5" width="16" height="15" rx="3" stroke-width="1.8"/><path d="M8 3v4m8-4v4M4 10h16" stroke-width="1.8" stroke-linecap="round"/><path d="M9 15h6" stroke-width="2" stroke-linecap="round"/></svg>',
  planned: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="9" stroke-width="1.8"/><path d="M12 7v5l3.2 2" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"/></svg>',
  all: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M9 6h11M9 12h11M9 18h11M4.5 6h.01M4.5 12h.01M4.5 18h.01" stroke-width="2" stroke-linecap="round"/></svg>',
  completed: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="9" stroke-width="1.8"/><path d="m8 12 2.7 2.7L16.5 9" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>',
}

const smartNav = [
  { to: '/admin', label: '今天', key: 'today', tone: 'bg-blue-500/12 text-blue-500', icon: icons.today },
  { to: '/admin/planned', label: '计划', key: 'planned', tone: 'bg-amber-500/12 text-amber-500', icon: icons.planned },
  { to: '/admin/all', label: '全部', key: 'all', tone: 'bg-violet-500/12 text-violet-500', icon: icons.all },
  { to: '/admin/completed', label: '已完成', key: 'completed', tone: 'bg-emerald-500/12 text-emerald-500', icon: icons.completed },
]
const adminNav = [
  { to: '/admin/users', label: '用户管理', tone: 'bg-blue-500/12 text-blue-500', icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><path d="M16 20v-1.5a4 4 0 0 0-4-4H7a4 4 0 0 0-4 4V20M9.5 10.5a3.5 3.5 0 1 0 0-7 3.5 3.5 0 0 0 0 7ZM21 20v-1.5a4 4 0 0 0-3-3.87M16.5 3.6a3.5 3.5 0 0 1 0 6.79" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>' },
  { to: '/admin/configs', label: '系统配置', tone: 'bg-violet-500/12 text-violet-500', icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="3" stroke-width="1.8"/><path d="M19.4 15a1.7 1.7 0 0 0 .34 1.88l.06.06-2.83 2.83-.06-.06A1.7 1.7 0 0 0 15 19.4a1.7 1.7 0 0 0-1 .6 1.7 1.7 0 0 0-.4 1.1V21h-4v-.09A1.7 1.7 0 0 0 8.6 19.4a1.7 1.7 0 0 0-1.88.34l-.06.06-2.83-2.83.06-.06A1.7 1.7 0 0 0 4.6 15a1.7 1.7 0 0 0-.6-1 1.7 1.7 0 0 0-1.1-.4H3v-4h.09A1.7 1.7 0 0 0 4.6 8.6a1.7 1.7 0 0 0-.34-1.88l-.06-.06 2.83-2.83.06.06A1.7 1.7 0 0 0 9 4.6a1.7 1.7 0 0 0 1-.6 1.7 1.7 0 0 0 .4-1.1V3h4v.09A1.7 1.7 0 0 0 15.4 4.6a1.7 1.7 0 0 0 1.88-.34l.06-.06 2.83 2.83-.06.06A1.7 1.7 0 0 0 19.4 9c.13.38.35.72.65 1 .3.28.7.42 1.1.4H21v4h-.09A1.7 1.7 0 0 0 19.4 15Z" stroke-width="1.35" stroke-linecap="round" stroke-linejoin="round"/></svg>' },
  { to: '/admin/audit', label: '操作日志', tone: 'bg-amber-500/12 text-amber-500', icon: '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor"><rect x="5" y="4" width="14" height="17" rx="2" stroke-width="1.8"/><path d="M9 4h6v3H9zM9 11h6M9 15h4" stroke-width="1.8" stroke-linecap="round"/></svg>' },
]

const userInitial = computed(() => (authStore.user?.username || 'R').charAt(0).toUpperCase())
const greeting = computed(() => {
  const h = new Date().getHours()
  return h < 6 ? '夜深了' : h < 11 ? '早上好' : h < 14 ? '中午好' : h < 18 ? '下午好' : '晚上好'
})
const dayLabel = computed(() => new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }).format(new Date()))

watch(creatingList, async (open) => {
  if (open) {
    await nextTick()
    listInput.value?.focus()
  }
})
watch(() => route.fullPath, () => {
  mobileOpen.value = false
})

function isActive(to: string) {
  return to === '/admin' ? route.path === '/admin' : route.path === to
}
function listColor(color: string) {
  return ({ blue: '#3182f6', violet: '#8b5cf6', rose: '#f43f5e', amber: '#f59e0b', emerald: '#10b981' } as Record<string, string>)[color] || '#3182f6'
}
function openQuickAdd() {
  if (!route.path.startsWith('/admin') || ['notifications', 'channels', 'settings', 'profile'].some(x => route.path.includes(x))) router.push('/admin')
  mobileOpen.value = false
  setTimeout(() => window.dispatchEvent(new CustomEvent('open-quick-reminder')), 80)
}
async function submitList() {
  if (!newListName.value.trim()) return
  await createList({ name: newListName.value.trim(), color: ['blue', 'violet', 'rose', 'amber', 'emerald'][lists.value.length % 5] })
  newListName.value = ''
  creatingList.value = false
  await refreshNavigation()
}
async function refreshNavigation() {
  try {
    const [listRes, summaryRes] = await Promise.all([getLists(), getSummary()])
    lists.value = listRes.data.data || []
    summary.value = summaryRes.data.data || {}
  } catch {}
}
async function loadVersion() {
  try {
    const res = await getVersion()
    const info = res.data?.data
    appVersion.value = info?.version || ''
    appBuildTime.value = info?.buildTime || ''
  } catch { /* a version label must never block the main application */ }
}
function logout() {
  stopRealtime?.()
  authStore.logout()
  router.push('/login')
}

let stopRealtime: (() => void) | undefined
let audioContext: AudioContext | undefined

function unlockReminderSound() {
  if (!audioContext) audioContext = new AudioContext()
  if (audioContext.state === 'suspended') void audioContext.resume()
}

function playReminderSound() {
  if (!audioContext || audioContext.state !== 'running') return
  const start = audioContext.currentTime
  const gain = audioContext.createGain()
  gain.gain.setValueAtTime(0.0001, start)
  gain.gain.exponentialRampToValueAtTime(0.12, start + 0.025)
  gain.gain.exponentialRampToValueAtTime(0.0001, start + 0.8)
  gain.connect(audioContext.destination)

  ;[
    { frequency: 659.25, offset: 0, duration: 0.32 },
    { frequency: 987.77, offset: 0.2, duration: 0.5 },
  ].forEach(note => {
    const oscillator = audioContext!.createOscillator()
    oscillator.type = 'sine'
    oscillator.frequency.setValueAtTime(note.frequency, start + note.offset)
    oscillator.connect(gain)
    oscillator.start(start + note.offset)
    oscillator.stop(start + note.offset + note.duration)
  })
}

function handleRealtimeEvent(event: ReminderRealtimeEvent) {
  if (event.type !== 'notification.created' || !event.notification) return
  window.dispatchEvent(new CustomEvent('reminder-realtime', { detail: event }))
  window.dispatchEvent(new CustomEvent('reminder-data-changed'))
  realtimeToast.value = ''
  setTimeout(() => {
    realtimeToast.value = event.notification?.type === 'channel_failed'
      ? event.notification.title
      : `提醒：${event.notification?.title || ''}`
  }, 0)
  if (event.notification.type === 'reminder_due') playReminderSound()
}

function refreshWhenVisible() {
  if (document.visibilityState === 'visible') {
    refreshNavigation()
    window.dispatchEvent(new CustomEvent('reminder-realtime-resume'))
  }
}

onMounted(() => {
  refreshNavigation()
  loadVersion()
  window.addEventListener('reminder-data-changed', refreshNavigation)
  document.addEventListener('pointerdown', unlockReminderSound, { once: true })
  document.addEventListener('keydown', unlockReminderSound, { once: true })
  document.addEventListener('visibilitychange', refreshWhenVisible)
  if (authStore.token) stopRealtime = connectReminderEvents(authStore.token, handleRealtimeEvent)
})
onBeforeUnmount(() => {
  stopRealtime?.()
  audioContext?.close()
  window.removeEventListener('reminder-data-changed', refreshNavigation)
  document.removeEventListener('pointerdown', unlockReminderSound)
  document.removeEventListener('keydown', unlockReminderSound)
  document.removeEventListener('visibilitychange', refreshWhenVisible)
})
</script>

<style scoped>
.ambient { position: fixed; inset: 0; z-index: -20; overflow: hidden; pointer-events: none; background: rgb(var(--color-background)); }
.ambient-orb { position: absolute; border-radius: 999px; filter: blur(110px); opacity: .16; }
.ambient-orb-one { width: 32rem; height: 32rem; left: 12%; top: -18rem; background: #60a5fa; }
.ambient-orb-two { width: 28rem; height: 28rem; right: -10rem; bottom: -12rem; background: #a78bfa; opacity: .11; }
.sidebar { @apply fixed inset-y-0 left-0 z-50 w-[292px] -translate-x-full border-r border-border/80 bg-surface/90 backdrop-blur-2xl transition-transform duration-300 lg:translate-x-0; }
.sidebar-open { @apply translate-x-0; }
.logo-mark { @apply flex h-11 w-11 items-center justify-center rounded-[15px] bg-gradient-to-br from-blue-500 to-indigo-500 text-white shadow-[0_10px_30px_-10px_rgba(59,130,246,.75)]; }
.version-badge { @apply rounded-full border border-brand-500/25 bg-brand-500/10 px-2 py-0.5 text-[11px] font-bold leading-none text-brand-600 dark:text-brand-300; }
.new-reminder-button { @apply flex h-12 items-center gap-3 rounded-2xl bg-gradient-to-r from-blue-500 to-indigo-500 px-3.5 text-sm font-bold text-white shadow-[0_14px_30px_-14px_rgba(59,130,246,.8)] transition hover:-translate-y-0.5 hover:brightness-105 active:translate-y-0; }
.nav-caption { @apply px-3 pb-2 text-[10px] font-extrabold uppercase tracking-[.18em] text-muted-foreground/75; }
.nav-row { @apply my-0.5 flex min-h-11 items-center gap-3 rounded-xl px-3 text-sm font-semibold text-foreground/80 transition-all hover:bg-muted hover:text-foreground; }
.nav-row-active { @apply bg-brand-500/10 text-brand-600 shadow-[inset_3px_0_0_rgb(var(--color-brand-500))] dark:text-brand-300; }
.nav-symbol { @apply flex h-8 w-8 items-center justify-center rounded-[10px]; }
.nav-symbol :deep(svg), .mobile-tab :deep(svg) { width: 18px; height: 18px; }
.nav-count { @apply min-w-6 rounded-full bg-muted px-1.5 py-0.5 text-center text-[10px] font-bold text-muted-foreground; }
.tiny-add { @apply flex h-7 w-7 items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground; }
.sub-link { @apply ml-11 block rounded-lg px-3 py-2 text-xs font-medium text-muted-foreground hover:bg-muted hover:text-foreground; }
.topbar { @apply sticky top-0 z-30 flex h-[76px] items-center border-b border-border/70 bg-background/75 px-4 backdrop-blur-xl sm:px-6 lg:px-8 xl:px-10; }
.icon-button { @apply flex h-10 w-10 items-center justify-center rounded-xl text-muted-foreground transition hover:bg-muted hover:text-foreground; }
.avatar { @apply flex h-9 w-9 items-center justify-center rounded-xl bg-gradient-to-br from-sky-400 to-indigo-500 text-sm font-extrabold text-white shadow-sm; }
.mobile-tabbar { @apply fixed inset-x-3 bottom-3 z-40 flex h-16 items-center justify-around rounded-[22px] border border-white/30 bg-surface/90 px-2 shadow-[0_18px_50px_-18px_rgba(15,23,42,.45)] backdrop-blur-2xl lg:hidden; }
.mobile-tab { @apply flex w-14 flex-col items-center gap-0.5 text-muted-foreground transition; }
.mobile-tab span { @apply flex h-7 w-7 items-center justify-center; }
.mobile-tab small { @apply text-[10px] font-semibold; }
.mobile-tab.active { @apply text-brand-500; }
.mobile-add { @apply -mt-7 flex h-14 w-14 items-center justify-center rounded-full border-4 border-background bg-gradient-to-br from-blue-500 to-indigo-500 text-white shadow-[0_12px_28px_-8px_rgba(59,130,246,.8)]; }
.page-enter-active, .page-leave-active { transition: opacity .18s ease, transform .18s ease; }
.page-enter-from { opacity: 0; transform: translateY(5px); }
.page-leave-to { opacity: 0; transform: translateY(-3px); }
@media (min-width: 1024px) { .mobile-only { display: none !important; } }
@media (max-width: 767px), (hover: none) and (pointer: coarse) {
  .ambient-orb { display: none; }
  .sidebar, .topbar, .mobile-tabbar {
    backdrop-filter: none;
    -webkit-backdrop-filter: none;
  }
}
</style>
