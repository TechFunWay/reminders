<template>
  <Teleport to="body">
    <Transition name="alert">
      <div
        v-if="open"
        class="fixed inset-0 z-[130] flex items-center justify-center bg-black/55 p-4 pb-[calc(1rem+env(safe-area-inset-bottom))] pt-[calc(1rem+env(safe-area-inset-top))]"
      >
        <section
          role="alertdialog"
          aria-modal="true"
          :aria-label="title"
          class="surface-modal w-full max-w-sm overflow-hidden rounded-2xl"
        >
          <!-- 顶部品牌条：站内提醒是「有事找你」，不是普通提示，视觉上要一眼看到 -->
          <div class="flex items-center gap-3 border-b border-border/70 bg-gradient-to-r from-blue-500/12 to-indigo-500/12 px-5 py-3.5">
            <span class="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-gradient-to-br from-blue-500 to-indigo-500 text-white shadow-[0_8px_20px_-8px_rgba(59,130,246,.9)]">
              <svg class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.9" d="M18 8a6 6 0 0 0-12 0c0 7-3 7-3 9h18c0-2-3-2-3-9ZM10 21h4" />
              </svg>
            </span>
            <div class="min-w-0 flex-1">
              <p class="text-[11px] font-bold uppercase tracking-[0.18em] text-muted-foreground">提醒事项</p>
              <p class="truncate text-sm font-bold text-foreground">{{ timeLabel }}</p>
            </div>
            <span
              v-if="pendingCount > 0"
              class="shrink-0 rounded-full bg-brand-500/15 px-2 py-0.5 text-[11px] font-bold text-brand-600 dark:text-brand-300"
            >还有 {{ pendingCount }} 条</span>
          </div>

          <div class="px-5 pb-5 pt-4">
            <h2 class="text-lg font-bold leading-6 text-foreground">{{ title }}</h2>
            <p v-if="body" class="mt-1.5 text-[13px] leading-5 text-muted-foreground">{{ body }}</p>

            <p v-if="error" class="mt-3 text-[13px] text-rose-600 dark:text-rose-400">{{ error }}</p>

            <div class="mt-5 flex flex-col gap-2.5">
              <div class="flex gap-2.5">
                <button type="button" class="btn-ghost flex-1" :disabled="busy" @click="emit('snooze')">
                  {{ snoozeText }}
                </button>
                <button ref="completeButtonRef" type="button" class="btn-brand flex-1" :disabled="busy" @click="emit('complete')">
                  <svg v-if="busy" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
                  {{ busy ? '处理中…' : '完成' }}
                </button>
              </div>
              <button type="button" class="rounded-xl px-4 py-2 text-[13px] font-semibold text-muted-foreground transition-colors hover:bg-muted hover:text-foreground disabled:opacity-60" :disabled="busy" @click="emit('dismiss')">
                知道了
              </button>
            </div>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
// 站内提醒弹窗。
//
// 为什么要有它：站内通知以前只在右下角飘一条 toast，手机端那个位置既容易被
// 忽略、也容易被底部标签栏挡住——提醒到点却什么也没看见，等于没提醒。这里改成
// 屏幕中央的弹窗，必须由用户处理（完成 / 稍后提醒 / 知道了）才消失，PC 与手机
// 一致。
//
// 可访问性：role=alertdialog + 打开时自动聚焦「完成」，Esc 等同于「知道了」。
import { computed, nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  open: boolean
  title?: string
  body?: string
  createdAt?: string
  /** 队列里还排着多少条，让用户知道后面还有。 */
  pendingCount?: number
  busy?: boolean
  error?: string
}>(), {
  title: '',
  body: '',
  createdAt: '',
  pendingCount: 0,
  busy: false,
  error: '',
})

const emit = defineEmits<{
  complete: []
  snooze: []
  dismiss: []
}>()

const snoozeText = '稍后提醒'
const completeButtonRef = ref<HTMLButtonElement | null>(null)

const timeLabel = computed(() => {
  if (!props.createdAt) return '到点了'
  const at = new Date(props.createdAt)
  if (Number.isNaN(at.getTime())) return '到点了'
  return new Intl.DateTimeFormat('zh-CN', {
    month: 'numeric', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false,
  }).format(at)
})

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  event.stopPropagation()
  if (props.busy) return
  emit('dismiss')
}

watch(() => props.open, async (open) => {
  if (open) {
    window.addEventListener('keydown', onKeydown)
    await nextTick()
    completeButtonRef.value?.focus()
    return
  }
  window.removeEventListener('keydown', onKeydown)
}, { immediate: true })

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.alert-enter-active,
.alert-leave-active {
  transition: opacity 0.2s ease;
}
.alert-enter-from,
.alert-leave-to {
  opacity: 0;
}
.alert-enter-active section,
.alert-leave-active section {
  transition: transform 0.2s ease;
}
.alert-enter-from section,
.alert-leave-to section {
  transform: scale(0.95) translateY(8px);
}
</style>
