<template>
  <div class="mx-auto max-w-6xl">
    <section class="mb-6 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between">
      <div>
        <div class="mb-2 flex items-center gap-2 text-xs font-bold uppercase tracking-[.18em] text-muted-foreground">
          <span class="h-2 w-2 rounded-full" :style="{ background: accentColor }"></span>
          {{ eyebrow }}
        </div>
        <h1 class="font-display text-3xl font-extrabold tracking-tight sm:text-4xl">{{ pageTitle }}</h1>
        <p class="mt-2 text-sm text-muted-foreground">{{ subtitle }}</p>
      </div>
      <label class="search-box">
        <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><circle cx="11" cy="11" r="7" stroke-width="1.8"/><path d="m16.5 16.5 4 4" stroke-width="1.8" stroke-linecap="round"/></svg>
        <input v-model="search" placeholder="搜索提醒…" @input="debouncedLoad" />
        <kbd class="hidden rounded-md border border-border bg-muted px-1.5 py-0.5 text-[10px] sm:inline">⌘ K</kbd>
      </label>
    </section>

    <section v-if="view !== 'completed'" class="quick-card" :class="{ 'quick-card-expanded': quickExpanded }">
      <div class="flex items-start gap-3">
        <button class="mt-0.5 flex h-8 w-8 shrink-0 items-center justify-center rounded-full border-2 border-brand-500/25 text-brand-500" aria-label="新提醒">
          <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M12 5v14M5 12h14" stroke-width="2" stroke-linecap="round"/></svg>
        </button>
        <div class="min-w-0 flex-1">
          <input
            ref="quickInput"
            v-model="quick.title"
            class="w-full bg-transparent text-[17px] font-semibold outline-none placeholder:font-medium placeholder:text-muted-foreground/65"
            placeholder="添加一条提醒…"
            maxlength="200"
            @keydown.enter.prevent="submitQuick"
            @focus="quickFocused = true"
          />
          <p v-if="quick.notes" class="mt-1 truncate text-xs text-muted-foreground">{{ quick.notes }}</p>
        </div>
        <button v-if="quick.title" class="btn-primary !h-9 !px-4" :disabled="saving" @click="submitQuick">{{ saving ? '保存中' : '添加' }}</button>
      </div>

      <div v-if="quickFocused || quick.title" class="mt-4 border-t border-border/60 pt-4">
        <div class="flex flex-wrap items-center gap-2">
          <button class="quick-chip" :class="{ active: preset === 'today' }" @click="setPreset('today')">今天</button>
          <button class="quick-chip" :class="{ active: preset === 'tomorrow' }" @click="setPreset('tomorrow')">明天</button>
          <button class="quick-chip" :class="{ active: preset === 'weekend' }" @click="setPreset('weekend')">本周末</button>
          <button class="quick-chip" :class="{ active: quick.priority === 3 }" @click="quick.priority = quick.priority === 3 ? 0 : 3">
            <span class="text-rose-500">!!!</span> 重要
          </button>
          <button class="quick-chip" :class="{ active: quickExpanded }" @click="quickExpanded = !quickExpanded">
            <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M12 6v.01M12 12v.01M12 18v.01" stroke-width="3" stroke-linecap="round"/></svg>
            更多
          </button>
          <span v-if="quick.due_at" class="ml-auto text-xs font-semibold text-brand-600 dark:text-brand-300">{{ formatDue(quick.due_at) }}</span>
        </div>

        <Transition name="expand">
          <div v-if="quickExpanded" class="mt-4 grid gap-3 rounded-2xl bg-muted/65 p-4 sm:grid-cols-2 lg:grid-cols-4">
            <label class="field-label">日期时间<input v-model="quickDate" type="datetime-local" class="compact-input" @change="preset = ''" /></label>
            <label class="field-label">清单<select v-model.number="quick.list_id" class="compact-input"><option v-for="list in lists" :key="list.id" :value="list.id">{{ list.name }}</option></select></label>
            <label class="field-label">重复<select v-model="quick.repeat_rule" class="compact-input"><option value="none">不重复</option><option value="daily">每天</option><option value="weekly">每周</option><option value="monthly">每月</option><option value="yearly">每年</option></select></label>
            <label class="field-label">备注<input v-model="quick.notes" class="compact-input" placeholder="可选" maxlength="5000" /></label>
            <div class="sm:col-span-2 lg:col-span-4">
              <p class="field-label mb-2">通知方式</p>
              <div class="flex flex-wrap gap-2">
                <button v-for="channel in selectableChannels" :key="channel.channel" class="channel-pill" :class="{ selected: isQuickChannelSelected(channel.channel) }" :disabled="channel.channel !== 'inapp' && (!channel.bound || channel.status !== 'active')" @click="toggleQuickChannel(channel.channel)">
                  {{ channel.label }}
                </button>
              </div>
            </div>
          </div>
        </Transition>
      </div>
    </section>

    <div class="mb-3 mt-7 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <h2 class="text-sm font-bold">{{ items.length }} 项</h2>
        <span v-if="overdueCount" class="rounded-full bg-rose-500/10 px-2 py-0.5 text-[11px] font-bold text-rose-500">{{ overdueCount }} 项已过期</span>
      </div>
      <button class="text-xs font-semibold text-muted-foreground hover:text-foreground" @click="loadData">刷新</button>
    </div>

    <section v-if="loading" class="space-y-3">
      <div v-for="n in 4" :key="n" class="h-20 animate-pulse rounded-2xl bg-surface/80"></div>
    </section>

    <section v-else-if="items.length" class="space-y-2.5">
      <article
        v-for="item in items"
        :key="item.id"
        class="reminder-row group"
        :class="{ 'reminder-completed': !!item.completed_at, 'reminder-overdue': isOverdue(item) }"
      >
        <button class="complete-button" :aria-label="item.completed_at ? '恢复提醒' : '完成提醒'" @click.stop="toggleComplete(item)">
          <svg v-if="item.completed_at" viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="m6 12 4 4 8-9" stroke-width="2.4" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </button>
        <button class="min-w-0 flex-1 text-left" @click="editItem(item)">
          <div class="flex items-center gap-2">
            <h3 class="truncate text-[15px] font-bold">{{ item.title }}</h3>
            <span v-if="item.priority" class="priority-mark" :class="`priority-${item.priority}`">{{ '!'.repeat(item.priority) }}</span>
            <span v-if="item.repeat_rule !== 'none'" class="tiny-badge">循环</span>
          </div>
          <p v-if="item.notes" class="mt-1 truncate text-xs text-muted-foreground">{{ item.notes }}</p>
          <div class="mt-2 flex flex-wrap items-center gap-2 text-[11px] font-medium text-muted-foreground">
            <span class="inline-flex items-center gap-1" :class="{ 'text-rose-500': isOverdue(item) }">
              <svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="9" stroke-width="1.8"/><path d="M12 7v5l3 2" stroke-width="1.8" stroke-linecap="round"/></svg>
              {{ item.due_at ? formatDue(item.snoozed_until || item.due_at) : '无日期' }}
            </span>
            <span class="h-1 w-1 rounded-full bg-border"></span>
            <span>{{ item.list_name }}</span>
            <span v-if="item.channels.length" class="h-1 w-1 rounded-full bg-border"></span>
            <span v-if="item.channels.length">{{ item.channels.map(channelName).join(' · ') }}</span>
          </div>
        </button>
        <div class="row-actions">
          <button v-if="!item.completed_at && item.due_at" class="row-action" title="稍后提醒" @click="snooze(item, 10)">
            <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M12 8v4l3 2M5 3l-2 3m16-3 2 3M12 22a8 8 0 1 0 0-16 8 8 0 0 0 0 16Z" stroke-width="1.8" stroke-linecap="round"/></svg>
          </button>
          <button class="row-action hover:!text-rose-500" title="删除" @click="removeItem(item)">
            <svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor"><path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg>
          </button>
        </div>
      </article>
    </section>

    <section v-else class="empty-state">
      <div class="empty-art" aria-hidden="true">
        <span class="empty-ring"></span>
        <span class="empty-check"><svg viewBox="0 0 24 24" class="h-8 w-8" fill="none" stroke="currentColor"><path d="m6 12 4 4 8-9" stroke-width="2.3" stroke-linecap="round" stroke-linejoin="round"/></svg></span>
        <span class="empty-dot dot-one"></span><span class="empty-dot dot-two"></span><span class="empty-dot dot-three"></span>
      </div>
      <h3 class="mt-5 text-lg font-extrabold">{{ emptyTitle }}</h3>
      <p class="mt-1 max-w-sm text-sm leading-6 text-muted-foreground">{{ emptyText }}</p>
      <button v-if="view !== 'completed'" class="btn-primary mt-5" @click="focusQuickAdd">添加第一条提醒</button>
    </section>

    <Transition name="drawer">
      <div v-if="editorOpen" class="fixed inset-0 z-[70] flex justify-end">
        <div class="absolute inset-0 bg-slate-950/25 backdrop-blur-sm" @click="closeEditor"></div>
        <aside class="editor-panel">
          <div class="flex items-center justify-between border-b border-border px-6 py-5">
            <div>
              <p class="text-[10px] font-extrabold uppercase tracking-[.18em] text-muted-foreground">提醒详情</p>
              <h2 class="mt-1 text-lg font-extrabold">{{ editing.id ? '编辑提醒' : '新提醒' }}</h2>
            </div>
            <button class="row-action !opacity-100" aria-label="关闭" @click="closeEditor"><svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="m6 6 12 12M18 6 6 18" stroke-width="2" stroke-linecap="round"/></svg></button>
          </div>
          <form class="flex h-[calc(100%-81px)] flex-col" @submit.prevent="saveEditor">
            <div class="flex-1 space-y-6 overflow-y-auto p-6">
              <label class="editor-field"><span>标题</span><input v-model="editing.title" autofocus maxlength="200" placeholder="要提醒什么？" /></label>
              <label class="editor-field"><span>备注</span><textarea v-model="editing.notes" rows="4" maxlength="5000" placeholder="补充一些细节…"></textarea></label>
              <div class="grid grid-cols-2 gap-3">
                <label class="editor-field"><span>日期时间</span><input v-model="editingDate" type="datetime-local" /></label>
                <label class="editor-field"><span>清单</span><select v-model.number="editing.list_id"><option v-for="list in lists" :key="list.id" :value="list.id">{{ list.name }}</option></select></label>
                <label class="editor-field"><span>优先级</span><select v-model.number="editing.priority"><option :value="0">无</option><option :value="1">低</option><option :value="2">中</option><option :value="3">高</option></select></label>
                <label class="editor-field"><span>重复</span><select v-model="editing.repeat_rule"><option value="none">不重复</option><option value="daily">每天</option><option value="weekly">每周</option><option value="monthly">每月</option><option value="yearly">每年</option></select></label>
              </div>
              <div>
                <p class="mb-3 text-xs font-bold text-muted-foreground">通知方式</p>
                <div class="grid grid-cols-2 gap-2">
                  <button v-for="channel in selectableChannels" :key="channel.channel" type="button" class="channel-option" :class="{ selected: isEditingChannelSelected(channel.channel) }" :disabled="channel.channel !== 'inapp' && (!channel.bound || channel.status !== 'active')" @click="toggleEditingChannel(channel.channel)">
                    <span class="channel-check"><svg v-if="isEditingChannelSelected(channel.channel)" viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor"><path d="m6 12 4 4 8-9" stroke-width="2.5" stroke-linecap="round"/></svg></span>
                    <span class="text-left"><strong>{{ channel.label }}</strong><small>{{ channel.bound || channel.channel === 'inapp' ? '可用' : '未绑定' }}</small></span>
                  </button>
                </div>
                <RouterLink to="/admin/channels" class="mt-3 inline-flex text-xs font-semibold text-brand-600 hover:underline dark:text-brand-300">管理通知方式 →</RouterLink>
              </div>
            </div>
            <div class="flex items-center gap-3 border-t border-border bg-surface/80 p-5 backdrop-blur">
              <button type="button" class="btn-secondary flex-1" @click="closeEditor">取消</button>
              <button class="btn-primary flex-1" :disabled="saving || !editing.title.trim()">{{ saving ? '保存中…' : '保存提醒' }}</button>
            </div>
          </form>
        </aside>
      </div>
    </Transition>

    <Toast :message="toast.message" :type="toast.type" />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute } from 'vue-router'
import Toast from '../components/Toast.vue'
import {
  completeReminder, createReminder, deleteReminder, getChannelStatuses, getLists, getReminders,
  restoreReminder, snoozeReminder, updateReminder,
  type ChannelStatus, type ReminderChannel, type ReminderItem, type ReminderList, type SaveReminderInput
} from '../api/reminder'

const route = useRoute()
const items = ref<ReminderItem[]>([])
const lists = ref<ReminderList[]>([])
const selectableChannels = ref<ChannelStatus[]>([])
const loading = ref(true)
const saving = ref(false)
const search = ref('')
const quickInput = ref<HTMLInputElement | null>(null)
const quickFocused = ref(false)
const quickExpanded = ref(false)
const preset = ref('')
const editorOpen = ref(false)
const clockNow = ref(Date.now())
const toast = reactive<{ message: string; type: 'success' | 'error' }>({ message: '', type: 'success' })

const blankInput = (): SaveReminderInput => ({
  title: '', notes: '', list_id: 0, priority: 0, due_at: null,
  all_day: false, repeat_rule: 'none', channels: ['inapp'],
})
const quick = reactive<SaveReminderInput>(blankInput())
const editing = reactive<SaveReminderInput & { id?: number }>(blankInput())
let quickChannelsTouched = false

const view = computed(() => String(route.meta.reminderView || 'today'))
const selectedListID = computed(() => route.name === 'ReminderList' ? Number(route.params.id) : undefined)
const selectedList = computed(() => lists.value.find(x => x.id === selectedListID.value))
const pageTitle = computed(() => selectedList.value?.name || String(route.meta.title || '今天'))
const eyebrow = computed(() => selectedList.value ? '我的清单' : view.value === 'today' ? '此刻最重要' : '提醒事项')
const accentColor = computed(() => selectedList.value ? listColor(selectedList.value.color) : view.value === 'completed' ? '#10b981' : '#3182f6')
const subtitle = computed(() => {
  if (selectedList.value) return `${selectedList.value.open_count || 0} 件事情正在等你`
  if (view.value === 'today') return new Intl.DateTimeFormat('zh-CN', { month: 'long', day: 'numeric', weekday: 'long' }).format(new Date())
  if (view.value === 'planned') return '按时间整理好接下来的安排'
  if (view.value === 'completed') return '每一个勾选，都是向前一步'
  return '所有未完成的提醒都在这里'
})
const overdueCount = computed(() => items.value.filter(isOverdue).length)
const emptyTitle = computed(() => view.value === 'completed' ? '还没有完成记录' : view.value === 'today' ? '今天轻松了' : '这里还没有提醒')
const emptyText = computed(() => view.value === 'completed' ? '完成的提醒会安静地收在这里。' : '把脑海里惦记的事情记下来，剩下的交给 Reminder。')

const quickDate = computed({
  get: () => toLocalInput(quick.due_at),
  set: value => { quick.due_at = fromLocalInput(value) },
})
const editingDate = computed({
  get: () => toLocalInput(editing.due_at),
  set: value => { editing.due_at = fromLocalInput(value) },
})

let searchTimer: ReturnType<typeof setTimeout> | undefined
function debouncedLoad() {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(loadItems, 250)
}

watch(() => route.fullPath, loadRouteData)

async function loadData() {
  loading.value = true
  try {
    const [listRes, channelRes] = await Promise.all([getLists(), getChannelStatuses()])
    lists.value = listRes.data.data || []
    selectableChannels.value = channelRes.data.data || []
    if (!quickChannelsTouched) {
      quick.channels = selectableChannels.value
        .filter(channel => channel.channel === 'inapp' || (channel.bound && channel.status === 'active'))
        .map(channel => channel.channel)
      if (!quick.channels.length) quick.channels = ['inapp']
    }
    if (!quick.list_id) quick.list_id = selectedListID.value || lists.value.find(x => x.is_default)?.id || lists.value[0]?.id || 0
    await loadItems()
  } catch (err: any) {
    showToast(err.response?.data?.message || '读取提醒失败', 'error')
  } finally {
    loading.value = false
  }
}
async function loadRouteData() {
  loading.value = true
  try {
    if (selectedListID.value && !lists.value.some(list => list.id === selectedListID.value)) {
      await loadData()
      return
    }
    await loadItems()
  } catch (err: any) {
    showToast(err.response?.data?.message || '读取提醒失败', 'error')
  } finally {
    loading.value = false
  }
}
async function loadItems() {
  const res = await getReminders({ view: view.value, list_id: selectedListID.value, q: search.value || undefined })
  items.value = res.data.data || []
}
async function submitQuick() {
  if (!quick.title?.trim() || saving.value) return
  saving.value = true
  try {
    await createReminder({ ...quick, title: quick.title.trim() })
    quickChannelsTouched = false
    Object.assign(quick, blankInput(), { list_id: selectedListID.value || lists.value.find(x => x.is_default)?.id || lists.value[0]?.id || 0 })
    preset.value = ''
    quickExpanded.value = false
    quickFocused.value = false
    await loadItems()
    changed()
    showToast('提醒已添加')
  } catch (err: any) {
    showToast(err.response?.data?.message || '添加失败', 'error')
  } finally {
    saving.value = false
    quickInput.value?.focus()
  }
}
function setPreset(kind: string) {
  preset.value = preset.value === kind ? '' : kind
  if (!preset.value) {
    quick.due_at = null
    return
  }
  const d = new Date()
  d.setSeconds(0, 0)
  if (kind === 'today') d.setHours(20, 0)
  if (kind === 'tomorrow') { d.setDate(d.getDate() + 1); d.setHours(9, 0) }
  if (kind === 'weekend') { const add = (6 - d.getDay() + 7) % 7 || 7; d.setDate(d.getDate() + add); d.setHours(9, 0) }
  quick.due_at = d.toISOString()
}
function toggleQuickChannel(channel: ReminderChannel) {
  quickChannelsTouched = true
  const list = quick.channels || []
  quick.channels = list.includes(channel) ? list.filter(x => x !== channel) : [...list, channel]
  if (!quick.channels.length) quick.channels = ['inapp']
}
function isQuickChannelSelected(channel: ReminderChannel) { return (quick.channels || []).includes(channel) }
function toggleEditingChannel(channel: ReminderChannel) {
  const list = editing.channels || []
  editing.channels = list.includes(channel) ? list.filter(x => x !== channel) : [...list, channel]
  if (!editing.channels.length) editing.channels = ['inapp']
}
function isEditingChannelSelected(channel: ReminderChannel) { return (editing.channels || []).includes(channel) }
function editItem(item: ReminderItem) {
  Object.assign(editing, {
    id: item.id, title: item.title, notes: item.notes, list_id: item.list_id,
    priority: item.priority, due_at: item.due_at, all_day: item.all_day,
    repeat_rule: item.repeat_rule, channels: [...item.channels], version: item.version,
  })
  editorOpen.value = true
}
function closeEditor() { editorOpen.value = false }
async function saveEditor() {
  if (!editing.id || !editing.title?.trim()) return
  saving.value = true
  try {
    const { id, ...payload } = editing
    await updateReminder(id, payload)
    editorOpen.value = false
    await loadItems()
    changed()
    showToast('提醒已保存')
  } catch (err: any) {
    showToast(err.response?.data?.message || '保存失败', 'error')
  } finally { saving.value = false }
}
async function toggleComplete(item: ReminderItem) {
  try {
    if (item.completed_at) await restoreReminder(item.id)
    else await completeReminder(item.id)
    await loadItems()
    changed()
    showToast(item.completed_at ? '已恢复提醒' : item.repeat_rule !== 'none' ? '已完成，下一次提醒已安排' : '做得好，已完成')
  } catch (err: any) { showToast(err.response?.data?.message || '操作失败', 'error') }
}
async function removeItem(item: ReminderItem) {
  if (!window.confirm(`删除“${item.title}”？`)) return
  try {
    await deleteReminder(item.id)
    await loadItems()
    changed()
    showToast('提醒已删除')
  } catch (err: any) { showToast(err.response?.data?.message || '删除失败', 'error') }
}
async function snooze(item: ReminderItem, minutes: number) {
  const until = new Date(Date.now() + minutes * 60_000)
  try {
    await snoozeReminder(item.id, until.toISOString())
    await loadItems()
    changed()
    showToast(`已稍后 ${minutes} 分钟提醒`)
  } catch (err: any) { showToast(err.response?.data?.message || '设置稍后提醒失败', 'error') }
}
function focusQuickAdd() {
  quickFocused.value = true
  quickInput.value?.focus()
  quickInput.value?.scrollIntoView({ behavior: 'smooth', block: 'center' })
}
function isOverdue(item: ReminderItem) { return !!item.due_at && !item.completed_at && new Date(item.snoozed_until || item.due_at).getTime() < clockNow.value }
function formatDue(value: string | null | undefined) {
  if (!value) return ''
  const d = new Date(value)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const tomorrow = new Date(now); tomorrow.setDate(now.getDate() + 1)
  const day = sameDay ? '今天' : d.toDateString() === tomorrow.toDateString() ? '明天' : new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(d)
  return `${day} ${d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })}`
}
function toLocalInput(value?: string | null) {
  if (!value) return ''
  const d = new Date(value)
  const offset = d.getTimezoneOffset()
  return new Date(d.getTime() - offset * 60_000).toISOString().slice(0, 16)
}
function fromLocalInput(value: string) { return value ? new Date(value).toISOString() : null }
function channelName(ch: ReminderChannel) { return ({ inapp: '站内', email: '邮件', sms: '短信', feishu: '飞书', qq: 'QQ', dingtalk: '钉钉' } as Record<string, string>)[ch] || ch }
function listColor(color: string) { return ({ blue: '#3182f6', violet: '#8b5cf6', rose: '#f43f5e', amber: '#f59e0b', emerald: '#10b981' } as Record<string, string>)[color] || '#3182f6' }
function changed() { window.dispatchEvent(new CustomEvent('reminder-data-changed')) }
function showToast(message: string, type: 'success' | 'error' = 'success') { toast.message = ''; setTimeout(() => Object.assign(toast, { message, type }), 0) }

let clockTimer: ReturnType<typeof setInterval> | undefined
function refreshRealtimeData() {
  clockNow.value = Date.now()
  void loadItems()
}

onMounted(() => {
  loadData()
  clockTimer = setInterval(() => { clockNow.value = Date.now() }, 60_000)
  window.addEventListener('open-quick-reminder', focusQuickAdd)
  window.addEventListener('reminder-realtime', refreshRealtimeData)
  window.addEventListener('reminder-realtime-resume', refreshRealtimeData)
  window.addEventListener('keydown', onShortcut)
})
onBeforeUnmount(() => {
  window.removeEventListener('open-quick-reminder', focusQuickAdd)
  window.removeEventListener('reminder-realtime', refreshRealtimeData)
  window.removeEventListener('reminder-realtime-resume', refreshRealtimeData)
  window.removeEventListener('keydown', onShortcut)
  clearTimeout(searchTimer)
  clearInterval(clockTimer)
})
function onShortcut(e: KeyboardEvent) {
  if (e.key.toLowerCase() === 'n' && !e.metaKey && !e.ctrlKey && !(e.target instanceof HTMLInputElement) && !(e.target instanceof HTMLTextAreaElement)) {
    e.preventDefault(); focusQuickAdd()
  }
  if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
    e.preventDefault(); (document.querySelector('.search-box input') as HTMLInputElement)?.focus()
  }
}
</script>

<style scoped>
.search-box { @apply flex h-11 w-full items-center gap-2 rounded-2xl border border-border/80 bg-surface/80 px-3.5 text-muted-foreground shadow-sm backdrop-blur transition focus-within:border-brand-500/40 focus-within:ring-4 focus-within:ring-brand-500/10 sm:w-64; }
.search-box input { @apply min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground/65; }
.quick-card { @apply relative overflow-hidden rounded-[24px] border border-border/70 bg-surface/90 p-4 shadow-[0_20px_50px_-30px_rgba(15,23,42,.35)] backdrop-blur-xl transition-all sm:p-5; }
.quick-card::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 3px; background: linear-gradient(#3b82f6, #6366f1); opacity: .75; }
.quick-chip { @apply inline-flex h-8 items-center gap-1.5 rounded-xl border border-border/70 bg-surface px-3 text-xs font-semibold text-muted-foreground transition hover:border-brand-500/30 hover:text-brand-600 disabled:opacity-40; }
.quick-chip.active { @apply border-brand-500/25 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.field-label { @apply block text-[11px] font-bold text-muted-foreground; }
.compact-input { @apply mt-1.5 h-9 w-full rounded-xl border border-border bg-surface px-3 text-xs font-medium text-foreground outline-none focus:border-brand-500/50; }
.channel-pill { @apply rounded-xl border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-muted-foreground transition disabled:cursor-not-allowed disabled:opacity-35; }
.channel-pill.selected { @apply border-brand-500/30 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.reminder-row { @apply relative flex min-h-[78px] items-center gap-3 overflow-hidden rounded-2xl border border-border/65 bg-surface/85 px-4 py-3.5 shadow-[0_6px_24px_-20px_rgba(15,23,42,.5)] backdrop-blur transition-all hover:-translate-y-0.5 hover:border-brand-500/20 hover:shadow-[0_16px_36px_-26px_rgba(15,23,42,.65)]; content-visibility: auto; contain-intrinsic-size: 78px; }
.reminder-row::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 2px; background: transparent; }
.reminder-overdue::before { background: #f43f5e; }
.reminder-completed { @apply opacity-60; }
.reminder-completed h3 { @apply line-through; }
.complete-button { @apply flex h-6 w-6 shrink-0 items-center justify-center rounded-full border-2 border-brand-500/35 text-white transition hover:scale-110 hover:border-brand-500 hover:bg-brand-500; }
.reminder-completed .complete-button { @apply border-emerald-500 bg-emerald-500; }
.priority-mark { @apply text-[11px] font-black tracking-[-.12em]; }
.priority-1 { @apply text-blue-500; }.priority-2 { @apply text-amber-500; }.priority-3 { @apply text-rose-500; }
.tiny-badge { @apply rounded-md bg-violet-500/10 px-1.5 py-0.5 text-[9px] font-extrabold text-violet-500; }
.row-actions { @apply flex shrink-0 items-center gap-1 opacity-100 transition sm:opacity-0 sm:group-hover:opacity-100; }
.row-action { @apply flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition hover:bg-muted hover:text-foreground; }
.empty-state { @apply mt-4 flex min-h-[360px] flex-col items-center justify-center rounded-[28px] border border-dashed border-border bg-surface/35 px-6 text-center; }
.empty-art { @apply relative h-28 w-28; }
.empty-ring { @apply absolute inset-2 rounded-full border border-brand-500/20 bg-brand-500/5; }
.empty-check { @apply absolute left-1/2 top-1/2 flex h-14 w-14 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-indigo-500 text-white shadow-[0_16px_35px_-12px_rgba(59,130,246,.65)]; }
.empty-dot { @apply absolute h-2.5 w-2.5 rounded-full; }.dot-one { @apply left-0 top-5 bg-amber-400; }.dot-two { @apply right-1 top-1 bg-violet-400; }.dot-three { @apply bottom-0 right-6 bg-emerald-400; }
.editor-panel { @apply relative z-10 h-full w-full max-w-[510px] border-l border-border bg-background/95 shadow-2xl backdrop-blur-2xl; }
.editor-field { @apply block text-xs font-bold text-muted-foreground; }
.editor-field input, .editor-field textarea, .editor-field select { @apply mt-2 w-full rounded-2xl border border-border bg-surface px-4 py-3 text-sm font-medium text-foreground outline-none transition placeholder:text-muted-foreground/55 focus:border-brand-500/45 focus:ring-4 focus:ring-brand-500/10; }
.editor-field textarea { @apply resize-none leading-6; }
.channel-option { @apply flex items-center gap-3 rounded-2xl border border-border bg-surface p-3 text-muted-foreground transition disabled:opacity-35; }
.channel-option.selected { @apply border-brand-500/30 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.channel-option strong { @apply block text-xs; }.channel-option small { @apply mt-0.5 block text-[10px] opacity-70; }
.channel-check { @apply flex h-5 w-5 items-center justify-center rounded-full border border-current; }
.btn-primary { @apply inline-flex h-11 items-center justify-center rounded-2xl bg-gradient-to-r from-blue-500 to-indigo-500 px-5 text-sm font-bold text-white shadow-[0_12px_25px_-12px_rgba(59,130,246,.75)] transition hover:-translate-y-0.5 hover:brightness-105 disabled:opacity-50; }
.btn-secondary { @apply inline-flex h-11 items-center justify-center rounded-2xl border border-border bg-surface px-5 text-sm font-bold text-foreground transition hover:bg-muted; }
.expand-enter-active, .expand-leave-active { transition: all .2s ease; }.expand-enter-from, .expand-leave-to { opacity: 0; transform: translateY(-6px); }
.drawer-enter-active, .drawer-leave-active { transition: opacity .22s ease; }.drawer-enter-active .editor-panel, .drawer-leave-active .editor-panel { transition: transform .28s cubic-bezier(.2,.8,.2,1); }.drawer-enter-from, .drawer-leave-to { opacity: 0; }.drawer-enter-from .editor-panel, .drawer-leave-to .editor-panel { transform: translateX(100%); }
</style>
