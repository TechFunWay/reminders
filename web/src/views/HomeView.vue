<template>
  <div class="mx-auto max-w-6xl">
    <section class="mb-3 flex flex-col gap-2.5 sm:mb-6 sm:flex-row sm:items-end sm:justify-between sm:gap-4">
      <div class="min-w-0">
        <div class="mb-2 hidden items-center gap-2 text-xs font-bold uppercase tracking-[.18em] text-muted-foreground sm:flex">
          <span class="h-2 w-2 rounded-full" :style="{ background: accentColor }"></span>
          {{ eyebrow }}
        </div>
        <div class="flex flex-wrap items-baseline gap-x-2">
          <h1 class="font-display text-2xl font-extrabold tracking-tight sm:text-3xl lg:text-4xl">{{ pageTitle }}</h1>
          <p class="truncate text-xs text-muted-foreground sm:hidden">{{ subtitle }}</p>
        </div>
        <p class="mt-2 hidden text-sm text-muted-foreground sm:block">{{ subtitle }}</p>
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

      <div v-if="quickFocused || quick.title" class="mt-3 border-t border-border/60 pt-3 sm:mt-4 sm:pt-4">
        <div class="flex flex-wrap items-center gap-1.5 sm:gap-2">
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
          <div v-if="quickExpanded" class="mt-3 grid gap-2.5 rounded-2xl bg-muted/65 p-3 sm:mt-4 sm:grid-cols-2 sm:gap-3 sm:p-4 lg:grid-cols-4">
            <div class="field-label relative">
              日期时间
              <button ref="quickDateAnchor" type="button" class="compact-input mt-1.5 flex h-9 w-full items-center justify-between px-3 text-left text-xs font-medium" :class="{ '!border-brand-500/50': quickDateOpen }" @click="quickDateOpen = !quickDateOpen">
                <span :class="{ 'text-muted-foreground/60': !quick.due_at }">{{ quick.due_at ? formatFullDue(quick.due_at) : '选择日期' }}</span>
                <svg viewBox="0 0 24 24" class="h-3.5 w-3.5 text-muted-foreground" fill="none" stroke="currentColor"><rect x="3" y="5" width="18" height="16" rx="2" stroke-width="1.8"/><path d="M8 3v4m8-4v4M3 10h18" stroke-width="1.8" stroke-linecap="round"/></svg>
              </button>
              <LunarDatePicker v-if="quickDateOpen" v-model="quickDatePicked" with-time :anchor-el="quickDateAnchor" @commit="quickDateOpen = false" />
            </div>
            <label class="field-label">清单<select v-model.number="quick.list_id" class="compact-input"><option v-for="list in lists" :key="list.id" :value="list.id">{{ list.name }}</option></select></label>
            <label class="field-label">重复<select v-model="quick.repeat_rule" class="compact-input"><option value="none">不重复</option><option value="daily">每天</option><option value="weekly">每周</option><option value="monthly">每月</option><option value="yearly">每年</option></select></label>
            <label class="field-label">备注<input v-model="quick.notes" class="compact-input" placeholder="可选" maxlength="5000" /></label>
            <label class="field-label">按农历循环<select v-model="quick.calendar" class="compact-input" @change="upgradeCalendarToYearly(quick)"><option value="solar">公历</option><option value="lunar">农历</option></select></label>
            <label class="field-label">过期提醒频率<RepeatNotifySelect :model-value="quick.repeat_notify_minutes ?? 0" @update:model-value="quick.repeat_notify_minutes = $event" /></label>
            <p v-if="quick.calendar === 'lunar'" class="text-[10px] leading-4 text-muted-foreground sm:col-span-2 lg:col-span-4">已按农历循环：提醒按所选日期的农历月/日重复，腊月三十遇小年自动改为除夕当天，闰月日期遇无闰月年份自动用对应平月。</p>
            <div class="sm:col-span-2 lg:col-span-4">
              <p class="field-label mb-2">通知方式</p>
              <div class="flex flex-wrap gap-2">
                <button v-for="channel in selectableChannels" :key="channel.channel" class="channel-pill" :class="{ selected: isQuickChannelSelected(channel.channel) }" :disabled="channel.channel !== 'inapp' && (!channel.bound || channel.status !== 'active')" @click="toggleQuickChannel(channel.channel)">
                  {{ channel.label }}
                </button>
              </div>
            </div>
            <!-- 底部提交按钮：手机端填完下方字段后无需滚回顶部再点添加 -->
            <div class="sm:col-span-2 lg:col-span-4">
              <button class="btn-primary w-full" :disabled="saving || !quick.title?.trim()" @click="submitQuick">{{ saving ? '保存中…' : '添加提醒' }}</button>
            </div>
          </div>
        </Transition>
      </div>
    </section>

    <div class="mb-2 mt-5 flex items-center justify-between">
      <div class="flex items-center gap-2">
        <h2 class="text-sm font-bold">{{ items.length }} 项</h2>
        <span v-if="overdueCount" class="rounded-full bg-rose-500/10 px-2 py-0.5 text-[11px] font-bold text-rose-500">{{ overdueCount }} 项已过期</span>
      </div>
      <button class="text-xs font-semibold text-muted-foreground hover:text-foreground" @click="loadData">刷新</button>
    </div>

    <section v-if="loading" class="space-y-3">
      <div v-for="n in 4" :key="n" class="h-20 animate-pulse rounded-2xl bg-surface/80"></div>
    </section>

    <section v-else-if="items.length" class="space-y-2">
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
            <h3 class="truncate text-sm font-bold sm:text-[15px]">{{ item.title }}</h3>
            <span v-if="item.priority" class="priority-mark" :class="`priority-${item.priority}`">{{ '!'.repeat(item.priority) }}</span>
            <span v-if="item.repeat_rule !== 'none'" class="tiny-badge">{{ item.calendar === 'lunar' ? `农历${ruleLabel(item.repeat_rule)}` : ruleLabel(item.repeat_rule) }}</span>
            <span v-if="item.repeat_notify_minutes" class="tiny-badge">过期提醒</span>
          </div>
          <p v-if="item.notes" class="mt-0.5 truncate text-xs text-muted-foreground sm:mt-1">{{ item.notes }}</p>
          <div class="mt-1 flex flex-wrap items-center gap-2 text-[11px] font-medium text-muted-foreground sm:mt-2">
            <span class="inline-flex items-center gap-1" :class="{ 'text-rose-500': isOverdue(item) }">
              <svg viewBox="0 0 24 24" class="h-3.5 w-3.5" fill="none" stroke="currentColor"><circle cx="12" cy="12" r="9" stroke-width="1.8"/><path d="M12 7v5l3 2" stroke-width="1.8" stroke-linecap="round"/></svg>
              {{ item.due_at ? formatDue(item.snoozed_until || item.due_at) : '无日期' }}
            </span>
            <span class="h-1 w-1 rounded-full bg-border"></span>
            <span>{{ item.list_name }}</span>
            <span v-if="item.channels?.length" class="h-1 w-1 rounded-full bg-border"></span>
            <span v-if="item.channels?.length">{{ item.channels.map(channelName).join(' · ') }}</span>
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
          <div class="flex items-center justify-between border-b border-border px-4 py-3 sm:px-6 sm:py-5">
            <div>
              <p class="text-[10px] font-extrabold uppercase tracking-[.18em] text-muted-foreground">提醒详情</p>
              <h2 class="mt-0.5 text-base font-extrabold sm:mt-1 sm:text-lg">{{ editing.id ? '编辑提醒' : '新提醒' }}</h2>
            </div>
            <button class="row-action !opacity-100" aria-label="关闭" @click="closeEditor"><svg viewBox="0 0 24 24" class="h-5 w-5" fill="none" stroke="currentColor"><path d="m6 6 12 12M18 6 6 18" stroke-width="2" stroke-linecap="round"/></svg></button>
          </div>
          <form class="flex min-h-0 flex-1 flex-col" @submit.prevent="saveEditor">
            <div class="min-h-0 flex-1 space-y-4 overflow-y-auto p-4 sm:space-y-6 sm:p-6">
              <label class="editor-field"><span>标题</span><input v-model="editing.title" autofocus maxlength="200" placeholder="要提醒什么？" /></label>
              <label class="editor-field"><span>备注</span><textarea v-model="editing.notes" rows="4" maxlength="5000" placeholder="补充一些细节…"></textarea></label>
              <div class="grid grid-cols-2 gap-3">
                <div class="editor-field relative">
                  <span>日期时间</span>
                  <button ref="editingDateAnchor" type="button" class="mt-1.5 flex w-full items-center justify-between rounded-xl border px-3 py-2 text-left text-sm font-medium sm:mt-2 sm:rounded-2xl sm:px-4 sm:py-3" :class="editingDateOpen ? 'border-brand-500/45 ring-4 ring-brand-500/10' : 'border-border'" @click="editingDateOpen = !editingDateOpen">
                    <span :class="{ 'text-muted-foreground/55': !editing.due_at }">{{ editing.due_at ? formatFullDue(editing.due_at) : '选择日期' }}</span>
                    <svg viewBox="0 0 24 24" class="h-4 w-4 text-muted-foreground" fill="none" stroke="currentColor"><rect x="3" y="5" width="18" height="16" rx="2" stroke-width="1.8"/><path d="M8 3v4m8-4v4M3 10h18" stroke-width="1.8" stroke-linecap="round"/></svg>
                  </button>
                  <LunarDatePicker v-if="editingDateOpen" v-model="editingDatePicked" with-time :anchor-el="editingDateAnchor" @commit="editingDateOpen = false" />
                </div>
                <label class="editor-field"><span>清单</span><select v-model.number="editing.list_id"><option v-for="list in lists" :key="list.id" :value="list.id">{{ list.name }}</option></select></label>
                <label class="editor-field"><span>优先级</span><select v-model.number="editing.priority"><option :value="0">无</option><option :value="1">低</option><option :value="2">中</option><option :value="3">高</option></select></label>
                <label class="editor-field"><span>重复</span><select v-model="editing.repeat_rule"><option value="none">不重复</option><option value="daily">每天</option><option value="weekly">每周</option><option value="monthly">每月</option><option value="yearly">每年</option></select></label>
                <label class="editor-field"><span>按农历循环</span><select v-model="editing.calendar" @change="upgradeCalendarToYearly(editing)"><option value="solar">公历</option><option value="lunar">农历</option></select></label>
                <label class="editor-field"><span>过期提醒频率</span><RepeatNotifySelect :model-value="editing.repeat_notify_minutes ?? 0" @update:model-value="editing.repeat_notify_minutes = $event" /></label>
                <p class="col-span-2 mt-1 text-[10px] leading-4 text-muted-foreground">“过期提醒频率”在到点未完成时按所选间隔重复通知，点击完成后停止；配合“每天”等重复规则可实现固定时间的每日催办。</p>
                <p v-if="editing.calendar === 'lunar'" class="col-span-2 -mt-3 text-[10px] leading-4 text-muted-foreground">“按农历循环”按所选日期对应的农历月/日重复，适合农历生日等纪念日期；腊月三十遇小年自动改为除夕当天，闰月日期遇无闰月年份自动用对应平月。</p>
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
                <div v-if="isEditingChannelSelected('email') && emailBindings.length > 1" class="mt-4 rounded-2xl bg-muted/65 p-3">
                  <p class="mb-2 text-xs font-bold text-muted-foreground">这个提醒发送到</p>
                  <div class="flex flex-wrap gap-2">
                    <button type="button" class="channel-pill" :class="{ selected: editingEmailTargets.length === 0 }" @click="clearEditingEmailTargets">全部接收邮箱</button>
                    <button v-for="binding in emailBindings" :key="binding.id" type="button" class="channel-pill" :class="{ selected: editingEmailTargets.includes(binding.id) }" @click="toggleEditingEmailTarget(binding.id)">
                      {{ binding.target_masked }}
                    </button>
                  </div>
                  <p class="mt-2 text-[10px] leading-4 text-muted-foreground">选择“全部接收邮箱”时不指定具体邮箱；所选邮箱被删除后会自动改发全部已启用的邮箱。</p>
                </div>
              </div>
            </div>
            <div class="flex items-center gap-3 border-t border-border bg-surface/80 p-3.5 backdrop-blur sm:p-5">
              <button type="button" class="btn-secondary flex-1" @click="closeEditor">取消</button>
              <button class="btn-primary flex-1" :disabled="saving || !editing.title.trim()">{{ saving ? '保存中…' : '保存提醒' }}</button>
            </div>
          </form>
        </aside>
      </div>
    </Transition>

    <Toast :message="toast.message" :type="toast.type" />

    <ConfirmDialog
      v-model="removeVisible"
      danger
      title="删除这条提醒？"
      :message="`「${removeTarget?.title || ''}」删除后无法恢复。`"
      confirm-text="删除"
      loading-text="删除中…"
      :loading="removeLoading"
      @confirm="confirmRemove"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, reactive, ref, watch } from 'vue'
import { RouterLink, useRoute, useRouter } from 'vue-router'
import Toast from '../components/Toast.vue'
import ConfirmDialog from '../components/ConfirmDialog.vue'
import LunarDatePicker from '../components/LunarDatePicker.vue'
import RepeatNotifySelect from '../components/RepeatNotifySelect.vue'
import {
  completeReminder, createReminder, deleteReminder, getChannelStatuses, getLists, getReminders,
  restoreReminder, snoozeReminder, updateReminder,
  type ChannelStatus, type ReminderChannel, type ReminderItem, type ReminderList, type SaveReminderInput
} from '../api/reminder'

const route = useRoute()
const router = useRouter()
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
  all_day: false, repeat_rule: 'none', calendar: 'solar', repeat_notify_minutes: 0, channels: ['inapp'],
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

const quickDateOpen = ref(false)
const editingDateOpen = ref(false)
const quickDateAnchor = ref<HTMLElement | null>(null)
const editingDateAnchor = ref<HTMLElement | null>(null)

const quickDatePicked = computed({
  get: () => (quick.due_at ? new Date(quick.due_at) : null),
  set: value => {
    quick.due_at = value ? toLocalIso(value) : null
    preset.value = ''
  },
})
const editingDatePicked = computed({
  get: () => (editing.due_at ? new Date(editing.due_at) : null),
  set: value => { editing.due_at = value ? toLocalIso(value) : null },
})

// toLocalIso 序列化为带本地时区偏移的 RFC3339 串（后端 time.Time 只认
// RFC3339），既保住所选墙钟时间又不会因缺时区后缀被拒。
function toLocalIso(d: Date) {
  const pad = (n: number) => String(n).padStart(2, '0')
  const offsetMinutes = -d.getTimezoneOffset()
  const sign = offsetMinutes >= 0 ? '+' : '-'
  const absOffset = Math.abs(offsetMinutes)
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}${sign}${pad(Math.floor(absOffset / 60))}:${pad(absOffset % 60)}`
}

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
    // 没设日期的提醒不会出现在按到期时间过滤的「今天 / 计划」视图里：留在原地
    // 重新拉取会让用户以为没保存成功（接口 200、列表却毫无变化），所以切到
    // 「全部」并说明原因；已经在「全部 / 清单」里时行为不变。
    const undated = !quick.due_at
    await createReminder({ ...quick, title: quick.title.trim() })
    quickChannelsTouched = false
    Object.assign(quick, blankInput(), { list_id: selectedListID.value || lists.value.find(x => x.is_default)?.id || lists.value[0]?.id || 0 })
    preset.value = ''
    quickExpanded.value = false
    quickFocused.value = false
    if (undated && !selectedListID.value && (view.value === 'today' || view.value === 'planned')) {
      await router.push({ name: 'AllReminders' })
      showToast('已保存到「全部」：这条提醒没有设置日期')
    } else {
      await loadItems()
      showToast('提醒已添加')
    }
    changed()
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
  if (!editing.channels.includes('email')) editing.channel_targets = undefined
}
function isEditingChannelSelected(channel: ReminderChannel) { return (editing.channels || []).includes(channel) }
const emailBindings = computed(() => selectableChannels.value.find(channel => channel.channel === 'email')?.bindings || [])
const editingEmailTargets = computed(() => editing.channel_targets?.email || [])
function toggleEditingEmailTarget(id: number) {
  const picked = new Set(editing.channel_targets?.email || [])
  if (picked.has(id)) picked.delete(id)
  else picked.add(id)
  editing.channel_targets = picked.size ? { email: [...picked] } : undefined
}
function clearEditingEmailTargets() { editing.channel_targets = undefined }

// 农历循环只对每月/每年重复生效（后端会拒绝其余组合）。用户直接选农历时
// 自动把重复规则升到每年（农历生日等纪念日的默认用法）；反之把重复规则
// 切走时回退公历，避免残留 lunar 导致保存被拒。
function upgradeCalendarToYearly(target: SaveReminderInput) {
  if (target.calendar === 'lunar' && target.repeat_rule !== 'monthly' && target.repeat_rule !== 'yearly') {
    target.repeat_rule = 'yearly'
  }
}
watch(() => quick.repeat_rule, rule => {
  if (rule !== 'monthly' && rule !== 'yearly') quick.calendar = 'solar'
})
watch(() => editing.repeat_rule, rule => {
  if (rule !== 'monthly' && rule !== 'yearly') editing.calendar = 'solar'
})

function editItem(item: ReminderItem) {
  Object.assign(editing, {
    id: item.id, title: item.title, notes: item.notes, list_id: item.list_id,
    priority: item.priority, due_at: item.due_at, all_day: item.all_day,
    repeat_rule: item.repeat_rule, calendar: item.calendar || 'solar',
    repeat_notify_minutes: item.repeat_notify_minutes || 0,
    channels: [...(item.channels || [])], version: item.version,
    lunar_anchor: item.lunar_anchor || '',
    channel_targets: item.channel_targets ? { ...item.channel_targets } : undefined,
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
// 删除确认走应用内浮层（见 ConfirmDialog）：原生 confirm 在飞牛 App 的网页
// 视图里是系统级弹窗，与应用样式割裂，手机端也容易被当成浏览器提示忽略。
const removeVisible = ref(false)
const removeTarget = ref<ReminderItem | null>(null)
const removeLoading = ref(false)

function removeItem(item: ReminderItem) {
  removeTarget.value = item
  removeVisible.value = true
}

async function confirmRemove() {
  const item = removeTarget.value
  if (!item) return
  removeLoading.value = true
  try {
    await deleteReminder(item.id)
    await loadItems()
    changed()
    showToast('提醒已删除')
    removeVisible.value = false
    removeTarget.value = null
  } catch (err: any) {
    showToast(err.response?.data?.message || '删除失败', 'error')
  } finally { removeLoading.value = false }
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
// 日期按钮用完整格式：选中后能直接核对年月日时分，避免跨年日期歧义。
function formatFullDue(value: string | null | undefined) {
  if (!value) return ''
  const d = new Date(value)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}年${d.getMonth() + 1}月${d.getDate()}日 ${pad(d.getHours())}:${pad(d.getMinutes())}`
}
function formatDue(value: string | null | undefined) {
  if (!value) return ''
  const d = new Date(value)
  const now = new Date()
  const sameDay = d.toDateString() === now.toDateString()
  const tomorrow = new Date(now); tomorrow.setDate(now.getDate() + 1)
  const day = sameDay ? '今天' : d.toDateString() === tomorrow.toDateString() ? '明天' : new Intl.DateTimeFormat('zh-CN', { month: 'numeric', day: 'numeric' }).format(d)
  return `${day} ${d.toLocaleTimeString('zh-CN', { hour: '2-digit', minute: '2-digit', hour12: false })}`
}
function channelName(ch: ReminderChannel) { return ({ inapp: '站内', email: '邮件', sms: '短信', feishu: '飞书', qq: 'QQ', dingtalk: '钉钉' } as Record<string, string>)[ch] || ch }
function ruleLabel(rule: string) { return ({ daily: '每天', weekly: '每周', monthly: '每月', yearly: '每年' } as Record<string, string>)[rule] || '循环' }
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
.search-box { @apply flex h-10 w-full items-center gap-2 rounded-2xl border border-border/80 bg-surface/80 px-3 text-muted-foreground shadow-sm backdrop-blur transition focus-within:border-brand-500/40 focus-within:ring-4 focus-within:ring-brand-500/10 sm:h-11 sm:w-64; }
.search-box input { @apply min-w-0 flex-1 bg-transparent text-sm text-foreground outline-none placeholder:text-muted-foreground/65; }
.quick-card { @apply relative overflow-hidden rounded-[24px] border border-border/70 bg-surface/90 p-3 shadow-[0_20px_50px_-30px_rgba(15,23,42,.35)] backdrop-blur-xl transition-all sm:p-5; }
.quick-card::before { content: ''; position: absolute; inset: 0 auto 0 0; width: 3px; background: linear-gradient(#3b82f6, #6366f1); opacity: .75; }
.quick-chip { @apply inline-flex h-7 items-center gap-1.5 rounded-lg border border-border/70 bg-surface px-2.5 text-[11px] font-semibold text-muted-foreground transition hover:border-brand-500/30 hover:text-brand-600 disabled:opacity-40 sm:h-8 sm:rounded-xl sm:px-3 sm:text-xs; }
.quick-chip.active { @apply border-brand-500/25 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.field-label { @apply block text-[11px] font-bold text-muted-foreground; }
.compact-input { @apply mt-1.5 h-9 w-full rounded-xl border border-border bg-surface px-3 text-xs font-medium text-foreground outline-none focus:border-brand-500/50; }
.channel-pill { @apply rounded-xl border border-border bg-surface px-3 py-1.5 text-xs font-semibold text-muted-foreground transition disabled:cursor-not-allowed disabled:opacity-35; }
.channel-pill.selected { @apply border-brand-500/30 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.reminder-row { @apply relative flex min-h-0 items-center gap-2.5 overflow-hidden rounded-2xl border border-border/65 bg-surface/85 px-3 py-2.5 shadow-[0_6px_24px_-20px_rgba(15,23,42,.5)] backdrop-blur transition-all hover:-translate-y-0.5 hover:border-brand-500/20 hover:shadow-[0_16px_36px_-26px_rgba(15,23,42,.65)] sm:gap-3 sm:px-4 sm:py-3.5; content-visibility: auto; contain-intrinsic-size: 64px; }
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
.empty-state { @apply mt-4 flex min-h-[230px] flex-col items-center justify-center rounded-[28px] border border-dashed border-border bg-surface/35 px-6 text-center sm:min-h-[280px]; }
.empty-art { @apply relative h-28 w-28; }
.empty-ring { @apply absolute inset-2 rounded-full border border-brand-500/20 bg-brand-500/5; }
.empty-check { @apply absolute left-1/2 top-1/2 flex h-14 w-14 -translate-x-1/2 -translate-y-1/2 items-center justify-center rounded-2xl bg-gradient-to-br from-blue-500 to-indigo-500 text-white shadow-[0_16px_35px_-12px_rgba(59,130,246,.65)]; }
.empty-dot { @apply absolute h-2.5 w-2.5 rounded-full; }.dot-one { @apply left-0 top-5 bg-amber-400; }.dot-two { @apply right-1 top-1 bg-violet-400; }.dot-three { @apply bottom-0 right-6 bg-emerald-400; }
.editor-panel { @apply relative z-10 flex h-full w-full max-w-[510px] flex-col border-l border-border bg-background/95 shadow-2xl backdrop-blur-2xl; }
.editor-field { @apply block text-xs font-bold text-muted-foreground; }
.editor-field input, .editor-field textarea, .editor-field select { @apply mt-1.5 w-full rounded-xl border border-border bg-surface px-3 py-2 text-sm font-medium text-foreground outline-none transition placeholder:text-muted-foreground/55 focus:border-brand-500/45 focus:ring-4 focus:ring-brand-500/10 sm:mt-2 sm:rounded-2xl sm:px-4 sm:py-3; }
.editor-field textarea { @apply resize-none leading-6; }
.channel-option { @apply flex items-center gap-3 rounded-2xl border border-border bg-surface p-2.5 text-muted-foreground transition disabled:opacity-35 sm:p-3; }
.channel-option.selected { @apply border-brand-500/30 bg-brand-500/10 text-brand-600 dark:text-brand-300; }
.channel-option strong { @apply block text-xs; }.channel-option small { @apply mt-0.5 block text-[10px] opacity-70; }
.channel-check { @apply flex h-5 w-5 items-center justify-center rounded-full border border-current; }
.btn-primary { @apply inline-flex h-10 items-center justify-center rounded-2xl bg-gradient-to-r from-blue-500 to-indigo-500 px-5 text-sm font-bold text-white shadow-[0_12px_25px_-12px_rgba(59,130,246,.75)] transition hover:-translate-y-0.5 hover:brightness-105 disabled:opacity-50 sm:h-11; }
.btn-secondary { @apply inline-flex h-10 items-center justify-center rounded-2xl border border-border bg-surface px-5 text-sm font-bold text-foreground transition hover:bg-muted sm:h-11; }
.expand-enter-active, .expand-leave-active { transition: all .2s ease; }.expand-enter-from, .expand-leave-to { opacity: 0; transform: translateY(-6px); }
.drawer-enter-active, .drawer-leave-active { transition: opacity .22s ease; }.drawer-enter-active .editor-panel, .drawer-leave-active .editor-panel { transition: transform .28s cubic-bezier(.2,.8,.2,1); }.drawer-enter-from, .drawer-leave-to { opacity: 0; }.drawer-enter-from .editor-panel, .drawer-leave-to .editor-panel { transform: translateX(100%); }
</style>
