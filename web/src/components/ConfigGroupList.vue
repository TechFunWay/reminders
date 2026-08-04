<template>
  <div class="space-y-6">
    <!-- Empty state -->
    <div v-if="groups.length === 0" class="surface rounded-2xl p-16 text-center">
      <div class="flex flex-col items-center gap-3 text-muted-foreground">
        <svg class="w-12 h-12 opacity-40" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z"/></svg>
        <span class="text-sm">暂无配置项</span>
      </div>
    </div>

    <!-- Group panels -->
    <div
      v-for="group in groups"
      :key="group.key"
      class="surface rounded-2xl overflow-hidden"
    >
      <div class="border-b border-border p-5 sm:px-6">
        <h2 class="text-base font-bold text-foreground">{{ group.title }}</h2>
      </div>
      <div class="divide-y divide-border">
        <div
          v-for="item in group.items"
          :key="item.key"
          class="flex flex-col sm:flex-row sm:items-center gap-3 p-5 sm:px-6 hover:bg-muted/40 transition-colors"
        >
          <div class="flex-1 min-w-0">
            <div class="flex items-center gap-2 flex-wrap">
              <span class="text-sm font-semibold text-foreground">{{ item.label }}</span>
              <code class="text-xs text-muted-foreground">{{ item.key }}</code>
              <span
                v-if="isDirty(item)"
                class="badge bg-amber-100 text-amber-700 dark:bg-amber-500/15 dark:text-amber-300"
              >未保存</span>
            </div>
            <div class="text-xs text-muted-foreground mt-1">{{ item.description || '—' }}</div>
          </div>
          <div class="flex items-center gap-2 sm:w-80">
            <!-- bool: accessible toggle switch -->
            <button
              v-if="item.type === 'bool'"
              type="button"
              role="switch"
              :aria-checked="getEditValue(item) === 'true'"
              :aria-label="item.label"
              :disabled="getSaving(item)"
              @click="toggleAndSaveBool(item)"
              :class="[
                'w-11 h-6 rounded-full transition-colors shrink-0 focus:outline-none focus:ring-4 focus:ring-brand-500/20',
                getEditValue(item) === 'true' ? 'bg-brand-500' : 'bg-muted',
                getSaving(item) ? 'opacity-60 cursor-not-allowed' : 'cursor-pointer',
              ]"
            >
              <span
                :class="[
                  'inline-block w-5 h-5 rounded-full bg-white shadow transform transition-transform mt-0.5',
                  getEditValue(item) === 'true' ? 'translate-x-5' : 'translate-x-0.5',
                ]"
              ></span>
            </button>

            <!-- select: native styled select -->
            <select
              v-else-if="item.type === 'select'"
              :value="getEditValue(item)"
              :disabled="getSaving(item)"
              @change="onSelectChange(item, ($event.target as HTMLSelectElement).value)"
              @keyup.enter="emitSave(item)"
              class="input-field !py-2 flex-1"
            >
              <option v-for="opt in item.options || []" :key="opt" :value="opt">
                {{ optionLabel(item.key, opt) }}
              </option>
            </select>

            <!-- int: number input -->
            <input
              v-else-if="item.type === 'int'"
              type="number"
              :value="getEditValue(item)"
              :disabled="getSaving(item)"
              @input="setEditValue(item, ($event.target as HTMLInputElement).value)"
              @keyup.enter="emitSave(item)"
              class="input-field !py-2 flex-1"
            />

            <!-- string: text input -->
            <input
              v-else
              type="text"
              :value="getEditValue(item)"
              :disabled="getSaving(item)"
              @input="setEditValue(item, ($event.target as HTMLInputElement).value)"
              @keyup.enter="emitSave(item)"
              class="input-field !py-2 flex-1"
            />

            <button
              v-if="item.type !== 'bool'"
              @click="emitSave(item)"
              :disabled="!isDirty(item) || getSaving(item)"
              class="btn-brand !px-4 !py-2 whitespace-nowrap"
            >
              {{ getSaving(item) ? '保存中…' : '保存' }}
            </button>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, reactive, watch } from 'vue'
import type { ConfigMeta } from '../api/config'

interface RowState {
  editValue: string
  saving: boolean
}

const props = defineProps<{
  items: ConfigMeta[]
}>()

const emit = defineEmits<{
  (e: 'save', key: string, value: string): void
}>()

const groupTitleMap: Record<string, string> = {
  general: '站点设置',
  access: '访问控制',
  appearance: '外观',
  system: '系统维护',
}

const selectOptionLabelMap: Record<string, Record<string, string>> = {
  theme_mode: {
    system: '跟随系统',
    light: '浅色',
    dark: '深色',
  },
}

function groupTitle(key: string): string {
  return groupTitleMap[key] || key
}

function optionLabel(itemKey: string, opt: string): string {
  return selectOptionLabelMap[itemKey]?.[opt] || opt
}

const rowState = reactive<Record<string, RowState>>({})

// Sync per-row state with the prop:
//  - for new keys, seed editValue from the current value
//  - when an item's value prop changes externally and the row is not in-flight,
//    pull the new value into editValue (parent confirming save or external reload)
watch(
  () => props.items,
  (newItems) => {
    for (const item of newItems) {
      const existing = rowState[item.key]
      if (!existing) {
        rowState[item.key] = { editValue: item.value, saving: false }
      } else if (!existing.saving && existing.editValue !== item.value) {
        existing.editValue = item.value
      }
    }
  },
  { immediate: true, deep: true },
)

function ensureState(item: ConfigMeta) {
  if (!rowState[item.key]) {
    rowState[item.key] = { editValue: item.value, saving: false }
  }
}

function getEditValue(item: ConfigMeta): string {
  return rowState[item.key]?.editValue ?? item.value
}

function getSaving(item: ConfigMeta): boolean {
  return rowState[item.key]?.saving ?? false
}

function setEditValue(item: ConfigMeta, value: string) {
  ensureState(item)
  rowState[item.key].editValue = value
}

function isDirty(item: ConfigMeta): boolean {
  return getEditValue(item) !== item.value
}

function toggleAndSaveBool(item: ConfigMeta) {
  ensureState(item)
  const current = getEditValue(item)
  const next = current === 'true' ? 'false' : 'true'
  rowState[item.key].editValue = next
  if (next !== item.value && !getSaving(item)) {
    rowState[item.key].saving = true
    emit('save', item.key, next)
  }
}

function onSelectChange(item: ConfigMeta, value: string) {
  setEditValue(item, value)
}

function emitSave(item: ConfigMeta) {
  ensureState(item)
  if (!isDirty(item) || getSaving(item)) return
  rowState[item.key].saving = true
  emit('save', item.key, getEditValue(item))
}

// Public methods so the parent can flip saving back off after the request
// resolves — without these, the save button stays disabled until the parent's
// items prop change cascades through the watcher.
defineExpose({
  markSaved(key: string) {
    if (rowState[key]) rowState[key].saving = false
  },
  markFailed(key: string) {
    if (rowState[key]) rowState[key].saving = false
  },
})

const groups = computed(() => {
  const map = new Map<string, ConfigMeta[]>()
  for (const item of props.items) {
    const g = item.group || 'general'
    if (!map.has(g)) map.set(g, [])
    map.get(g)!.push(item)
  }
  return Array.from(map.entries()).map(([key, items]) => ({
    key,
    title: groupTitle(key),
    items,
  }))
})
</script>
