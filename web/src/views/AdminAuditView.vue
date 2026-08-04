<template>
  <div class="page-container animate-fade-in">
    <div class="surface rounded-2xl overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 p-5 sm:p-6 border-b border-border">
        <div>
          <h2 class="text-lg font-bold text-foreground">操作日志</h2>
          <p class="text-sm text-muted-foreground">追踪系统中的关键操作记录</p>
        </div>
        <div class="flex flex-wrap items-center gap-2">
          <input
            v-model="filterUsername"
            type="text"
            placeholder="用户名"
            class="input-field !py-2 w-36"
            @keyup.enter="reload"
          />
          <input
            v-model="filterAction"
            type="text"
            placeholder="操作 (如 login)"
            class="input-field !py-2 w-44"
            @keyup.enter="reload"
          />
          <button @click="reload" class="btn-brand !px-4 !py-2">筛选</button>
          <button @click="handleExport" class="btn-ghost !px-4 !py-2">
            <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4"/></svg>
            导出
          </button>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs uppercase tracking-wider text-muted-foreground bg-muted/60">
              <th class="py-3.5 px-6 font-semibold">时间</th>
              <th class="py-3.5 px-4 font-semibold">用户</th>
              <th class="py-3.5 px-4 font-semibold">操作</th>
              <th class="py-3.5 px-4 font-semibold">对象</th>
              <th class="py-3.5 px-4 font-semibold">详情</th>
              <th class="py-3.5 px-6 font-semibold">IP</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="log in logs" :key="log.id" class="hover:bg-muted/50 transition-colors">
              <td class="py-3.5 px-6 text-muted-foreground whitespace-nowrap">{{ log.created_at }}</td>
              <td class="py-3.5 px-4">
                <div class="flex items-center gap-2.5">
                  <div class="w-7 h-7 rounded-full flex items-center justify-center text-white font-bold text-[11px] shrink-0" :class="avatarClass(log.username || '?')">
                    {{ (log.username || '?').charAt(0).toUpperCase() }}
                  </div>
                  <div>
                    <div class="font-medium text-foreground leading-tight">{{ log.username }}</div>
                    <div class="text-xs text-muted-foreground">#{{ log.user_id }}</div>
                  </div>
                </div>
              </td>
              <td class="py-3.5 px-4">
                <span class="badge" :class="actionClass(log.action)">{{ log.action }}</span>
              </td>
              <td class="py-3.5 px-4 text-muted-foreground">
                <span v-if="log.target_type">{{ log.target_type }}<span v-if="log.target_id" class="text-muted-foreground">/{{ log.target_id }}</span></span>
                <span v-else>—</span>
              </td>
              <td class="py-3.5 px-4 text-foreground/80 max-w-xs truncate">{{ log.detail || '—' }}</td>
              <td class="py-3.5 px-6">
                <code class="text-xs text-muted-foreground">{{ log.ip }}</code>
              </td>
            </tr>
            <tr v-if="logs.length === 0">
              <td colspan="6" class="py-16 text-center">
                <div class="flex flex-col items-center gap-3 text-muted-foreground">
                  <svg class="w-12 h-12 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2"/></svg>
                  <span class="text-sm">暂无日志记录</span>
                </div>
              </td>
            </tr>
          </tbody>
        </table>
      </div>

      <div class="flex flex-wrap items-center justify-between gap-3 p-5 sm:px-6 border-t border-border">
        <div class="text-sm text-muted-foreground">
          共 <span class="font-semibold text-foreground">{{ total }}</span> 条 · 第 {{ page }} / {{ totalPages }} 页
        </div>
        <div class="flex items-center gap-2">
          <button @click="prevPage" :disabled="page <= 1" class="btn-ghost !px-3 !py-2">上一页</button>
          <button @click="nextPage" :disabled="page >= totalPages" class="btn-ghost !px-3 !py-2">下一页</button>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getAuditLogs, exportAuditLogs } from '../api/audit'

const logs = ref<any[]>([])
const page = ref(1)
const pageSize = ref(20)
const total = ref(0)
const totalPages = computed(() => Math.ceil(total.value / pageSize.value) || 1)

const filterUsername = ref('')
const filterAction = ref('')

const avatarPalette = [
  'bg-gradient-to-br from-indigo-500 to-purple-500',
  'bg-gradient-to-br from-sky-500 to-blue-600',
  'bg-gradient-to-br from-emerald-500 to-teal-600',
  'bg-gradient-to-br from-amber-500 to-orange-600',
  'bg-gradient-to-br from-pink-500 to-rose-600',
  'bg-gradient-to-br from-violet-500 to-fuchsia-600',
]
function avatarClass(name: string) {
  let h = 0
  for (let i = 0; i < name.length; i++) h = name.charCodeAt(i) + ((h << 5) - h)
  return avatarPalette[Math.abs(h) % avatarPalette.length]
}

function actionClass(action: string) {
  const a = (action || '').toLowerCase()
  if (a.includes('login')) return 'bg-sky-100 text-sky-700 dark:bg-sky-500/15 dark:text-sky-300'
  if (a.includes('delete')) return 'bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300'
  if (a.includes('create') || a.includes('register')) return 'bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300'
  if (a.includes('update') || a.includes('reset') || a.includes('change')) return 'bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300'
  return 'bg-indigo-100 text-indigo-700 dark:bg-indigo-500/15 dark:text-indigo-300'
}

onMounted(load)

function queryParams() {
  return {
    page: page.value,
    pageSize: pageSize.value,
    username: filterUsername.value || undefined,
    action: filterAction.value || undefined,
  }
}

async function load() {
  try {
    const res = await getAuditLogs(queryParams())
    if (res.data?.code === 0) {
      logs.value = res.data.data?.items || []
      total.value = res.data.data?.total || 0
    }
  } catch {}
}

function reload() {
  page.value = 1
  load()
}

function prevPage() {
  if (page.value > 1) {
    page.value--
    load()
  }
}

function nextPage() {
  if (page.value < totalPages.value) {
    page.value++
    load()
  }
}

async function handleExport() {
  try {
    const res = await exportAuditLogs({ username: filterUsername.value || undefined, action: filterAction.value || undefined })
    const url = window.URL.createObjectURL(new Blob([res.data]))
    const a = document.createElement('a')
    a.href = url
    a.download = 'audit-log.csv'
    a.click()
    window.URL.revokeObjectURL(url)
  } catch {}
}
</script>
