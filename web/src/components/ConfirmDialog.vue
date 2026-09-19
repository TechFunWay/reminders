<template>
  <Teleport to="body">
    <Transition name="confirm">
      <div
        v-if="modelValue"
        class="fixed inset-0 z-[120] flex items-center justify-center bg-black/55 p-4"
        @click.self="handleCancel"
      >
        <section
          role="dialog"
          aria-modal="true"
          :aria-label="title"
          class="surface-modal w-full max-w-sm rounded-2xl p-5 sm:p-6"
        >
          <div class="flex items-start gap-3.5">
            <span
              class="mt-0.5 flex h-10 w-10 shrink-0 items-center justify-center rounded-xl"
              :class="danger ? 'bg-rose-500/12 text-rose-500' : 'bg-brand-500/12 text-brand-600 dark:text-brand-300'"
            >
              <svg v-if="danger" class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.9" d="M12 9v4m0 4h.01M10.3 3.8 2.5 17.2A2 2 0 0 0 4.2 20h15.6a2 2 0 0 0 1.7-2.8L13.7 3.8a2 2 0 0 0-3.4 0Z"/></svg>
              <svg v-else class="h-5 w-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.9" d="M12 8v4m0 4h.01M12 21a9 9 0 1 0 0-18 9 9 0 0 0 0 18Z"/></svg>
            </span>
            <div class="min-w-0 flex-1">
              <h3 class="text-base font-bold text-foreground">{{ title }}</h3>
              <p v-if="message" class="mt-1 text-[13px] leading-5 text-muted-foreground">{{ message }}</p>
              <slot />
            </div>
          </div>

          <div class="mt-5 flex gap-2.5">
            <button ref="cancelButtonRef" type="button" class="btn-ghost flex-1" :disabled="loading" @click="handleCancel">
              {{ cancelText }}
            </button>
            <button
              type="button"
              class="flex-1"
              :class="danger ? dangerButtonClass : 'btn-brand'"
              :disabled="loading"
              @click="handleConfirm"
            >
              <svg v-if="loading" class="h-4 w-4 animate-spin" fill="none" viewBox="0 0 24 24"><circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"/><path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"/></svg>
              {{ loading ? loadingText : confirmText }}
            </button>
          </div>
        </section>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
// 应用内确认框：替代原生 window.confirm / alert。原生弹窗在飞牛 App 的网页
// 视图里样式与系统不一致（还会带上网页地址），在手机端也容易被当成浏览器提示
// 而忽略；这里统一用应用自己的浮层，危险操作红色按钮 + 二次确认。
import { nextTick, onBeforeUnmount, ref, watch } from 'vue'

const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  message?: string
  confirmText?: string
  cancelText?: string
  danger?: boolean
  loading?: boolean
  loadingText?: string
}>(), {
  title: '确认操作',
  message: '',
  confirmText: '确认',
  cancelText: '取消',
  danger: false,
  loading: false,
  loadingText: '处理中…',
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  confirm: []
  cancel: []
}>()

// 危险按钮不复用 .btn-ghost/.btn-brand：删除类操作用实心红，和页面里
// 「本页已禁用」「退出登录」的玫瑰色语义保持一致。
const dangerButtonClass =
  'inline-flex items-center justify-center gap-2 rounded-xl bg-rose-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition-all hover:bg-rose-700 active:scale-[0.98] disabled:pointer-events-none disabled:opacity-60'

const cancelButtonRef = ref<HTMLButtonElement | null>(null)

function handleConfirm() {
  // 关闭时机交给调用方：确认后通常还要发请求，loading 期间不能关窗。
  if (props.loading) return
  emit('confirm')
}

function handleCancel() {
  if (props.loading) return
  emit('update:modelValue', false)
  emit('cancel')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key !== 'Escape') return
  event.stopPropagation()
  handleCancel()
}

// 打开时把焦点放到「取消」：替代原生 confirm 的浮层不能因为一次回车就把
// 提醒或用户删掉，破坏性动作必须显式点确认。同时接上 Esc 关闭。
watch(() => props.modelValue, async (open) => {
  if (open) {
    window.addEventListener('keydown', onKeydown)
    await nextTick()
    cancelButtonRef.value?.focus()
    return
  }
  window.removeEventListener('keydown', onKeydown)
}, { immediate: true })

onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.confirm-enter-active,
.confirm-leave-active {
  transition: opacity 0.18s ease;
}
.confirm-enter-from,
.confirm-leave-to {
  opacity: 0;
}
.confirm-enter-active section,
.confirm-leave-active section {
  transition: transform 0.18s ease;
}
.confirm-enter-from section,
.confirm-leave-to section {
  transform: scale(0.96);
}
</style>
