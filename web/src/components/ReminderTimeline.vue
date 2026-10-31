<template>
  <div>
    <!-- 浏览导航：吸在顶栏下方，翻日 / 回今天 / 跳转任意日期（过去与未来都可以） -->
    <div class="timeline-nav sticky z-20" :class="navTopClass">
      <div class="flex items-center gap-1.5 rounded-2xl border border-border/70 bg-surface p-1.5 shadow-[0_10px_30px_-22px_rgba(15,23,42,.55)] sm:gap-2">
        <button class="nav-chip" aria-label="前一天" @click="stepDay(-1)">
          <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m15 6-6 6 6 6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
          <span class="hidden sm:inline">前一天</span>
        </button>
        <button class="nav-chip" :class="{ active: pinnedKey === todayKey }" @click="goToday">今天</button>
        <button class="nav-chip" aria-label="后一天" @click="stepDay(1)">
          <span class="hidden sm:inline">后一天</span>
          <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m9 6 6 6-6 6" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </button>
        <button ref="pickerAnchor" class="nav-chip nav-chip-date ml-auto" @click="pickerOpen = !pickerOpen">
          <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><rect x="3" y="5" width="18" height="16" rx="2" stroke-width="1.8"/><path d="M8 3v4m8-4v4M3 10h18" stroke-width="1.8" stroke-linecap="round"/></svg>
          <span class="max-w-[7.5rem] truncate">{{ anchorLabel }}</span>
        </button>
        <button
          class="nav-chip nav-chip-order"
          :title="descending ? '当前最新在上，点击改为最早在上' : '当前最早在上，点击改为最新在上'"
          :aria-label="descending ? '切换为正序（最早在上）' : '切换为倒序（最新在上）'"
          @click="toggleOrder"
        >
          <!-- 倒序：近的在上（向下渐远）；正序：远的在上（向上渐远） -->
          <svg v-if="descending" viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M12 5v14m0 0-4-4m4 4 4-4" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M5 5h2M5 9h3.5" stroke-width="2" stroke-linecap="round"/></svg>
          <svg v-else viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M12 19V5m0 0-4 4m4-4 4 4" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/><path d="M5 19h2m-2-4h3.5" stroke-width="2" stroke-linecap="round"/></svg>
          <span class="hidden sm:inline">{{ descending ? '倒序' : '正序' }}</span>
        </button>
      </div>
      <LunarDatePicker v-if="pickerOpen" v-model="jumpPicked" :anchor-el="pickerAnchor" @commit="pickerOpen = false" />
    </div>

    <div class="relative mt-2">
      <div class="tl-rail" aria-hidden="true"></div>
      <section
        v-for="group in groups"
        :key="group.key"
        :ref="el => setGroupEl(group.key, el)"
        class="tl-group relative scroll-mt-[7.5rem] sm:scroll-mt-[9.5rem] lg:scroll-mt-[10.75rem]"
        :class="{ 'tl-group-pinned': group.key === pinnedKey }"
      >
        <header class="flex items-center gap-2 pb-2 pt-1">
          <span class="tl-node" :class="nodeClass(group.key)"></span>
          <h3 class="tl-date" :class="dateClass(group.key)">{{ group.label }}</h3>
          <span v-if="group.items.length" class="tl-count" :class="{ 'tl-count-today': group.key === todayKey }">{{ group.items.length }} 项</span>
        </header>

        <p v-if="!group.items.length" class="tl-empty">这天没有安排</p>
        <div v-else class="space-y-1.5">
          <article
            v-for="item in group.items"
            :key="item.id"
            class="tl-card"
            :class="{ 'tl-card-done': !!item.completed_at }"
          >
            <button class="tl-check" :aria-label="item.completed_at ? '恢复提醒' : '完成提醒'" @click.stop="$emit('toggle', item)">
              <svg v-if="item.completed_at" viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor"><path d="m6 12 4 4 8-9" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
            </button>
            <button class="min-w-0 flex-1 text-left" @click="$emit('edit', item)">
              <div class="flex min-w-0 items-center gap-1.5">
                <h4 class="min-w-0 truncate text-sm font-bold">{{ item.title }}</h4>
                <span v-if="item.priority" class="shrink-0 text-[11px] font-black tracking-[-.12em]" :class="priorityClass(item.priority)">{{ '!'.repeat(item.priority) }}</span>
              </div>
              <p class="mt-0.5 flex min-w-0 items-center gap-1.5 text-[11px] font-medium text-muted-foreground">
                <span class="shrink-0" :class="{ 'font-bold text-rose-500': isOverdue(item) }">{{ timeLabel(item) }}</span>
                <span class="h-1 w-1 shrink-0 rounded-full bg-border"></span>
                <span class="min-w-0 truncate">{{ item.list_name }}</span>
                <template v-if="item.repeat_rule !== 'none'">
                  <span class="h-1 w-1 shrink-0 rounded-full bg-border"></span>
                  <span class="shrink-0">{{ item.calendar === 'lunar' ? `农历${ruleLabel(item.repeat_rule)}` : ruleLabel(item.repeat_rule) }}</span>
                </template>
              </p>
            </button>
          </article>
        </div>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import LunarDatePicker from './LunarDatePicker.vue'
import type { ReminderItem } from '../api/reminder'
import { SK } from '../utils/storage-keys'

const props = withDefaults(defineProps<{
  /** 全部有日期的提醒（后端 timeline 视图：过去 + 未来，含已完成） */
  items: ReminderItem[]
  /** 当前时刻（毫秒），父组件每分钟刷新，逾期着色跟着走 */
  now?: number
}>(), { now: () => Date.now() })

defineEmits<{
  (e: 'edit', item: ReminderItem): void
  (e: 'toggle', item: ReminderItem): void
}>()

const DAY_MS = 86_400_000

// 本地时区的「YYYY-MM-DD」键：分组、相对标签、跳转定位全用它对齐。
function dayKey(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}`
}
function keyDate(key: string) {
  const [y, m, d] = key.split('-').map(Number)
  return new Date(y, m - 1, d)
}
function shiftKey(key: string, days: number) {
  const d = keyDate(key)
  d.setDate(d.getDate() + days)
  return dayKey(d)
}

const todayKey = ref(dayKey(new Date()))
// 跨天停留（挂一整夜）时「今天 / 明天」标签要跟着翻页。
let dayTimer: ReturnType<typeof setInterval> | undefined

// 浏览锚点：翻日与跳转日期都改它；锚点那天没有事件时也会插一个空节点，
// 保证「跳到某天」永远有落点。初始锚在今天。
const pinnedKey = ref(todayKey.value)

interface DayGroup { key: string; label: string; items: ReminderItem[] }

// 排序方向：默认倒序（最新在最上，过去往下滑），可点导航条按钮切正序；
// 偏好记在 localStorage（带应用前缀的键）。
const descending = ref(localStorage.getItem(SK.timelineDesc) !== '0')
function toggleOrder() {
  descending.value = !descending.value
  localStorage.setItem(SK.timelineDesc, descending.value ? '1' : '0')
}

const groups = computed<DayGroup[]>(() => {
  const buckets = new Map<string, ReminderItem[]>()
  for (const item of props.items) {
    if (!item.due_at) continue
    const key = dayKey(new Date(item.due_at))
    const bucket = buckets.get(key)
    if (bucket) bucket.push(item)
    else buckets.set(key, [item])
  }
  for (const bucket of buckets.values()) {
    bucket.sort((a, b) => {
      const diff = new Date(a.due_at || 0).getTime() - new Date(b.due_at || 0).getTime()
      return descending.value ? -diff : diff
    })
  }
  const out = [...buckets.keys()].sort((a, b) => (descending.value ? b.localeCompare(a) : a.localeCompare(b))).map(key => ({
    key,
    label: labelFor(key),
    items: buckets.get(key) as ReminderItem[],
  }))
  // 锚点日期没有事件：按当前排序插入空节点，让「这天没有安排」落在正确位置。
  if (!buckets.has(pinnedKey.value)) {
    const empty = { key: pinnedKey.value, label: labelFor(pinnedKey.value), items: [] as ReminderItem[] }
    const idx = descending.value
      ? out.findIndex(group => group.key < pinnedKey.value)
      : out.findIndex(group => group.key > pinnedKey.value)
    if (idx === -1) out.push(empty)
    else out.splice(idx, 0, empty)
  }
  return out
})

function labelFor(key: string) {
  const diff = Math.round((keyDate(key).getTime() - keyDate(todayKey.value).getTime()) / DAY_MS)
  if (diff === 0) return '今天'
  if (diff === 1) return '明天'
  if (diff === -1) return '昨天'
  if (diff === 2) return '后天'
  if (diff === -2) return '前天'
  const d = keyDate(key)
  const week = ['周日', '周一', '周二', '周三', '周四', '周五', '周六'][d.getDay()]
  const md = `${d.getMonth() + 1}月${d.getDate()}日 ${week}`
  return d.getFullYear() === new Date().getFullYear() ? md : `${d.getFullYear()}年${md}`
}

const anchorLabel = computed(() => {
  if (pinnedKey.value === todayKey.value) return '选择日期'
  const d = keyDate(pinnedKey.value)
  const md = `${d.getMonth() + 1}月${d.getDate()}日`
  return d.getFullYear() === new Date().getFullYear() ? md : `${d.getFullYear()}年${md}`
})

const jumpPicked = computed({
  get: () => keyDate(pinnedKey.value),
  set: value => {
    if (!value) return
    pinnedKey.value = dayKey(value)
  },
})

const pickerOpen = ref(false)
const pickerAnchor = ref<HTMLElement | null>(null)

function stepDay(days: number) {
  pinnedKey.value = shiftKey(pinnedKey.value, days)
}
function goToday() {
  pinnedKey.value = todayKey.value
}

// 吸顶导航的 top 与外层顶栏（h-12 / sm:h-16 / lg:h-[76px]）+ 主区内边距对齐。
const navTopClass = 'top-[3.6rem] sm:top-[5.5rem] lg:top-[6.25rem]'

const groupEls = new Map<string, HTMLElement>()
function setGroupEl(key: string, el: unknown) {
  if (el) groupEls.set(key, el as HTMLElement)
  else groupEls.delete(key)
}

function scrollToPinned(behavior: ScrollBehavior) {
  void nextTick(() => {
    groupEls.get(pinnedKey.value)?.scrollIntoView({ behavior, block: 'start' })
  })
}

// 锚点变化（翻日 / 跳转）就把那天真滚到眼前；切换排序方向时锚点没变，
// 但它的位置翻了，同样滚过去；数据首次到位后定位一次今天，
// 让时间线打开时停在「现在」（默认倒序：今天在上、过去往下翻）。
watch(pinnedKey, () => scrollToPinned('smooth'))
watch(descending, () => scrollToPinned('smooth'))
let located = false
watch(groups, list => {
  if (!located && list.length) {
    located = true
    scrollToPinned('auto')
  }
}, { immediate: true })

function isOverdue(item: ReminderItem) {
  return !!item.due_at && !item.completed_at && new Date(item.due_at).getTime() < props.now
}
function timeLabel(item: ReminderItem) {
  if (!item.due_at) return ''
  if (item.all_day) return '全天'
  const d = new Date(item.due_at)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${pad(d.getHours())}:${pad(d.getMinutes())}`
}
function priorityClass(priority: number) {
  return ({ 1: 'text-blue-500', 2: 'text-amber-500', 3: 'text-rose-500' } as Record<number, string>)[priority] || ''
}
function ruleLabel(rule: string) {
  return ({ daily: '每天', weekly: '每周', monthly: '每月', yearly: '每年' } as Record<string, string>)[rule] || '循环'
}
function nodeClass(key: string) {
  if (key === todayKey.value) return 'tl-node-today'
  return key < todayKey.value ? 'tl-node-past' : 'tl-node-future'
}
function dateClass(key: string) {
  if (key === todayKey.value) return 'text-brand-600 dark:text-brand-300'
  return key < todayKey.value ? 'text-muted-foreground' : 'text-foreground'
}

onMounted(() => {
  dayTimer = setInterval(() => { todayKey.value = dayKey(new Date()) }, 60_000)
})
onBeforeUnmount(() => {
  if (dayTimer !== undefined) clearInterval(dayTimer)
})
</script>

<style scoped>
/* 手机端不用毛玻璃（渲染性能底线）：导航条底色用不透明的 surface。 */
.nav-chip { @apply flex h-9 items-center gap-1 rounded-xl px-2.5 text-xs font-semibold text-muted-foreground transition hover:bg-muted hover:text-foreground; }
.nav-chip.active { @apply bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.nav-chip-date { @apply border border-border/70 bg-surface font-bold text-foreground; }
.nav-chip-order { @apply border border-border/70 bg-surface; }
.tl-rail { @apply absolute bottom-3 left-[8px] top-3 w-px bg-border/80; }
.tl-group { @apply pl-7; }
.tl-node { @apply absolute left-0 top-1.5 h-[17px] w-[17px] rounded-full border-2 border-border bg-surface transition; }
.tl-node-today { @apply border-brand-500 bg-brand-500 shadow-[0_0_0_4px_rgb(var(--color-brand-500)/0.15)]; }
.tl-node-future { @apply border-brand-400/70 bg-surface; }
.tl-node-past { @apply border-border bg-muted; }
.tl-group-pinned .tl-node { @apply border-brand-500; }
.tl-date { @apply text-sm font-extrabold tracking-tight; }
.tl-count { @apply rounded-full bg-muted px-1.5 py-0.5 text-[10px] font-bold text-muted-foreground; }
.tl-count-today { @apply bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.tl-empty { @apply rounded-xl border border-dashed border-border/80 px-3 py-2.5 text-xs text-muted-foreground; }
.tl-card { @apply flex items-center gap-2.5 rounded-xl border border-border/65 bg-surface/85 px-3 py-2.5 transition-all hover:border-brand-500/20 hover:shadow-[0_12px_28px_-24px_rgba(15,23,42,.7)] sm:px-3.5; }
.tl-card-done { @apply opacity-60; }
.tl-card-done h4 { @apply line-through; }
.tl-check { @apply flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 border-brand-500/35 text-white transition hover:scale-110 hover:border-brand-500 hover:bg-brand-500; }
.tl-card-done .tl-check { @apply border-emerald-500 bg-emerald-500; }
</style>
