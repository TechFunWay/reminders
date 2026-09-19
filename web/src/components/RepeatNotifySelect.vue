<template>
  <div class="flex items-center gap-1.5">
    <select :value="presetValue" class="w-full" @change="onPresetChange">
      <option v-for="option in options" :key="option.value" :value="option.value">{{ option.label }}</option>
      <option value="custom">自定义…</option>
    </select>
    <div v-if="customizing" class="flex shrink-0 items-center gap-1">
      <input
        v-model.number="customValue"
        type="number"
        min="1"
        max="44640"
        class="w-16"
        @change="applyCustom"
        @keydown.enter="applyCustom"
      />
      <select v-model="customUnit" @change="applyCustom">
        <option :value="1">分钟</option>
        <option :value="60">小时</option>
        <option :value="1440">天</option>
      </select>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'

const props = defineProps<{
  /** 过期提醒间隔（分钟），0 表示不提醒 */
  modelValue: number
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: number): void
}>()

const options = [
  { value: 0, label: '不过期提醒' },
  { value: 5, label: '每 5 分钟' },
  { value: 15, label: '每 15 分钟' },
  { value: 30, label: '每 30 分钟' },
  { value: 60, label: '每 1 小时' },
  { value: 120, label: '每 2 小时' },
  { value: 240, label: '每 4 小时' },
  { value: 1440, label: '每 1 天' },
]

const customizing = ref(false)
const customValue = ref(30)
const customUnit = ref(60)
const MAX_MINUTES = 60 * 24 * 31

const presetValue = computed(() => {
  if (customizing.value) return 'custom'
  return options.some(o => o.value === props.modelValue) ? props.modelValue : 'custom'
})

watch(() => props.modelValue, value => {
  if (value !== 0 && !options.some(o => o.value === value)) {
    // 服务端返回了自定义间隔：展开编辑框并换算默认单位
    enterCustom(value)
  } else if (value === 0) {
    customizing.value = false
  }
}, { immediate: true })

function onPresetChange(event: Event) {
  const raw = (event.target as HTMLSelectElement).value
  if (raw === 'custom') {
    enterCustom(props.modelValue || 60)
    applyCustom()
    return
  }
  customizing.value = false
  emit('update:modelValue', Number(raw))
}

function enterCustom(minutes: number) {
  customizing.value = true
  if (minutes > 0 && minutes % 1440 === 0) {
    customValue.value = minutes / 1440
    customUnit.value = 1440
  } else if (minutes > 0 && minutes % 60 === 0) {
    customValue.value = minutes / 60
    customUnit.value = 60
  } else if (minutes > 0) {
    customValue.value = minutes
    customUnit.value = 1
  }
}

function applyCustom() {
  const minutes = Math.round((customValue.value || 0) * customUnit.value)
  emit('update:modelValue', Math.max(0, Math.min(MAX_MINUTES, minutes)))
}
</script>

<style scoped>
select { @apply h-9 w-full rounded-xl border border-border bg-surface px-3 text-xs font-medium text-foreground outline-none focus:border-brand-500/50; }
input { @apply h-9 rounded-xl border border-border bg-surface px-2 text-xs font-medium text-foreground outline-none focus:border-brand-500/50; }
</style>
