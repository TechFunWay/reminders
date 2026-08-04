<template>
  <div class="page-container animate-fade-in">
    <!-- Stat cards -->
    <div class="grid grid-cols-2 lg:grid-cols-4 gap-4">
      <div class="surface rounded-2xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm text-muted-foreground">用户总数</div>
            <div class="text-2xl font-extrabold text-foreground mt-1">{{ total }}</div>
          </div>
          <div class="w-11 h-11 rounded-xl bg-brand-50 dark:bg-brand-500/15 text-brand-600 dark:text-brand-300 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a4 4 0 00-3-3.87M9 20H4v-2a4 4 0 013-3.87m6-3.13a4 4 0 10-4-4 4 4 0 004 4z"/></svg>
          </div>
        </div>
      </div>
      <div class="surface rounded-2xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm text-muted-foreground">管理员</div>
            <div class="text-2xl font-extrabold text-foreground mt-1">{{ adminCount }}</div>
          </div>
          <div class="w-11 h-11 rounded-xl bg-violet-50 dark:bg-violet-500/15 text-violet-600 dark:text-violet-300 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z"/></svg>
          </div>
        </div>
      </div>
      <div class="surface rounded-2xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm text-muted-foreground">已启用</div>
            <div class="text-2xl font-extrabold text-foreground mt-1">{{ activeCount }}</div>
          </div>
          <div class="w-11 h-11 rounded-xl bg-emerald-50 dark:bg-emerald-500/15 text-emerald-600 dark:text-emerald-300 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
          </div>
        </div>
      </div>
      <div class="surface rounded-2xl p-5">
        <div class="flex items-center justify-between">
          <div>
            <div class="text-sm text-muted-foreground">已禁用</div>
            <div class="text-2xl font-extrabold text-foreground mt-1">{{ Math.max(total - activeCount, 0) }}</div>
          </div>
          <div class="w-11 h-11 rounded-xl bg-rose-50 dark:bg-rose-500/15 text-rose-600 dark:text-rose-300 flex items-center justify-center">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728A9 9 0 015.636 5.636m12.728 12.728L5.636 5.636"/></svg>
          </div>
        </div>
      </div>
    </div>

    <!-- Main panel -->
    <div class="surface rounded-2xl overflow-hidden">
      <div class="flex flex-wrap items-center justify-between gap-3 p-5 sm:p-6 border-b border-border">
        <div>
          <h2 class="text-lg font-bold text-foreground">用户列表</h2>
          <p class="text-sm text-muted-foreground">管理系统中的所有账户</p>
        </div>
        <div class="flex items-center gap-2 w-full sm:w-auto">
          <div class="relative flex-1 sm:flex-none">
            <svg class="absolute left-3.5 top-1/2 -translate-y-1/2 w-4 h-4 text-muted-foreground pointer-events-none" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/></svg>
            <input
              v-model="search"
              type="text"
              placeholder="搜索用户名..."
              class="input-field !pl-10 sm:w-64"
              @keyup.enter="reload"
            />
          </div>
          <button @click="reload" class="btn-brand whitespace-nowrap">搜索</button>
        </div>
      </div>

      <div class="overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="text-left text-xs uppercase tracking-wider text-muted-foreground bg-muted/60">
              <th class="py-3.5 px-6 font-semibold">用户</th>
              <th class="py-3.5 px-4 font-semibold">角色</th>
              <th class="py-3.5 px-4 font-semibold">状态</th>
              <th class="py-3.5 px-4 font-semibold">创建时间</th>
              <th class="py-3.5 px-6 font-semibold text-right">操作</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-border">
            <tr v-for="user in users" :key="user.id" class="hover:bg-muted/50 transition-colors">
              <td class="py-3.5 px-6">
                <div class="flex items-center gap-3">
                  <div class="w-9 h-9 rounded-full flex items-center justify-center text-white font-bold text-xs shrink-0" :class="avatarClass(user.username)">
                    {{ user.username.charAt(0).toUpperCase() }}
                  </div>
                  <div>
                    <div class="font-semibold text-foreground">{{ user.username }}</div>
                    <div class="text-xs text-muted-foreground">ID #{{ user.id }}</div>
                  </div>
                </div>
              </td>
              <td class="py-3.5 px-4">
                <span v-if="user.role === 'admin'" class="badge bg-violet-100 text-violet-700 dark:bg-violet-500/15 dark:text-violet-300">
                  <svg class="w-3 h-3" fill="currentColor" viewBox="0 0 20 20"><path d="M10 1l2.39 4.84L18 6.66l-4 3.9.94 5.5L10 13.5l-4.94 2.56.94-5.5-4-3.9 5.61-.82L10 1z"/></svg>
                  管理员
                </span>
                <span v-else class="badge bg-muted text-muted-foreground">普通用户</span>
              </td>
              <td class="py-3.5 px-4">
                <span v-if="user.status === 1" class="badge bg-emerald-100 text-emerald-700 dark:bg-emerald-500/15 dark:text-emerald-300">
                  <span class="w-1.5 h-1.5 rounded-full bg-emerald-500"></span>启用
                </span>
                <span v-else class="badge bg-rose-100 text-rose-700 dark:bg-rose-500/15 dark:text-rose-300">
                  <span class="w-1.5 h-1.5 rounded-full bg-rose-500"></span>禁用
                </span>
              </td>
              <td class="py-3.5 px-4 text-muted-foreground whitespace-nowrap">{{ user.created_at }}</td>
              <td class="py-3.5 px-6">
                <div class="flex items-center justify-end gap-1">
                  <button @click="handleToggleStatus(user)" :title="user.status === 1 ? '禁用' : '启用'" class="icon-btn hover:text-amber-600 hover:bg-amber-50 dark:hover:bg-amber-500/10">
                    <svg v-if="user.status === 1" class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M18.364 18.364A9 9 0 005.636 5.636m12.728 12.728L5.636 5.636"/></svg>
                    <svg v-else class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
                  </button>
                  <button @click="handleResetPassword(user)" title="重置密码" class="icon-btn hover:text-brand-600 hover:bg-brand-50 dark:hover:bg-brand-500/10">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
                  </button>
                  <button @click="handleDelete(user)" title="删除" class="icon-btn hover:text-rose-600 hover:bg-rose-50 dark:hover:bg-rose-500/10">
                    <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"/></svg>
                  </button>
                </div>
              </td>
            </tr>
            <tr v-if="users.length === 0">
              <td colspan="5" class="py-16 text-center">
                <div class="flex flex-col items-center gap-3 text-muted-foreground">
                  <svg class="w-12 h-12 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M17 20h5v-2a4 4 0 00-3-3.87M9 20H4v-2a4 4 0 013-3.87m6-3.13a4 4 0 10-4-4 4 4 0 004 4z"/></svg>
                  <span class="text-sm">暂无用户数据</span>
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

    <!-- Reset password modal -->
    <transition enter-active-class="transition-opacity duration-200" enter-from-class="opacity-0" leave-active-class="transition-opacity duration-150" leave-to-class="opacity-0">
      <div v-if="resetModalVisible" class="fixed inset-0 z-50 flex items-center justify-center p-4 bg-foreground/50 backdrop-blur-sm" @click.self="resetModalVisible = false">
        <div class="surface rounded-2xl shadow-2xl p-6 w-full max-w-md animate-scale-in">
          <div class="flex items-center gap-3 mb-5">
            <div class="w-11 h-11 rounded-xl bg-brand-50 dark:bg-brand-500/15 text-brand-600 dark:text-brand-300 flex items-center justify-center">
              <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 7a2 2 0 012 2m4 0a6 6 0 01-7.743 5.743L11 17H9v2H7v2H4a1 1 0 01-1-1v-2.586a1 1 0 01.293-.707l5.964-5.964A6 6 0 1121 9z"/></svg>
            </div>
            <div>
              <h3 class="text-lg font-bold text-foreground">重置密码</h3>
              <p class="text-sm text-muted-foreground">用户：{{ resetTarget?.username }}</p>
            </div>
          </div>
          <div class="space-y-4">
            <div>
              <label class="block text-sm font-medium text-foreground mb-1.5">新密码</label>
              <input v-model="resetNewPassword" type="password" placeholder="请输入新密码" class="input-field" @keyup.enter="confirmResetPassword" />
            </div>
            <div v-if="resetMsg" class="text-sm text-destructive bg-destructive/10 px-3 py-2 rounded-lg">{{ resetMsg }}</div>
            <div class="flex gap-3 pt-1">
              <button @click="resetModalVisible = false" class="btn-ghost flex-1">取消</button>
              <button @click="confirmResetPassword" :disabled="resetLoading" class="btn-brand flex-1">{{ resetLoading ? '重置中...' : '确认重置' }}</button>
            </div>
          </div>
        </div>
      </div>
    </transition>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { getUsers, toggleUserStatus, deleteUser, resetUserPassword } from '../api/user'
import { passwordValidationError } from '../utils/password'

const users = ref<any[]>([])
const search = ref('')
const page = ref(1)
const pageSize = ref(10)
const total = ref(0)
const totalPages = computed(() => Math.ceil(total.value / pageSize.value) || 1)

const adminCount = computed(() => users.value.filter((u) => u.role === 'admin').length)
const activeCount = computed(() => users.value.filter((u) => u.status === 1).length)

const resetModalVisible = ref(false)
const resetTarget = ref<any>(null)
const resetNewPassword = ref('')
const resetLoading = ref(false)
const resetMsg = ref('')

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

onMounted(loadUsers)

async function loadUsers() {
  try {
    const res = await getUsers(page.value, pageSize.value, search.value || undefined)
    if (res.data?.code === 0) {
      users.value = res.data.data?.list || res.data.data?.items || []
      total.value = res.data.data?.total || 0
    }
  } catch {}
}

function reload() {
  page.value = 1
  loadUsers()
}

function prevPage() {
  if (page.value > 1) {
    page.value--
    loadUsers()
  }
}

function nextPage() {
  if (page.value < totalPages.value) {
    page.value++
    loadUsers()
  }
}

async function handleToggleStatus(user: any) {
  try {
    const res = await toggleUserStatus(user.id)
    if (res.data?.code === 0) await loadUsers()
  } catch {}
}

async function handleDelete(user: any) {
  if (!confirm(`确定删除用户 ${user.username}？`)) return
  try {
    const res = await deleteUser(user.id)
    if (res.data?.code === 0) await loadUsers()
  } catch {}
}

function handleResetPassword(user: any) {
  resetTarget.value = user
  resetNewPassword.value = ''
  resetMsg.value = ''
  resetModalVisible.value = true
}

async function confirmResetPassword() {
  const validationError = passwordValidationError(resetNewPassword.value)
  if (validationError) {
    resetMsg.value = validationError
    return
  }
  resetLoading.value = true
  resetMsg.value = ''
  try {
    const res = await resetUserPassword(resetTarget.value.id, resetNewPassword.value)
    if (res.data?.code === 0) {
      resetModalVisible.value = false
    } else {
      resetMsg.value = res.data?.message || '重置失败'
    }
  } catch (err: any) {
    resetMsg.value = err.response?.data?.message || '网络错误'
  } finally {
    resetLoading.value = false
  }
}
</script>

<style scoped>
.icon-btn {
  @apply p-2 rounded-lg text-muted-foreground transition-colors;
}
</style>
