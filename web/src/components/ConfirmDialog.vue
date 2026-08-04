<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center" @click.self="handleCancel">
        <div class="absolute inset-0 bg-black bg-opacity-50"></div>
        <div class="relative bg-surface text-foreground rounded-2xl shadow-xl p-6 w-full max-w-sm mx-4">
          <h3 class="text-lg font-bold text-foreground mb-2">{{ title }}</h3>
          <p class="text-sm text-muted-foreground mb-6">{{ message }}</p>
          <div class="flex space-x-3">
            <button
              @click="handleCancel"
              class="flex-1 px-4 py-2 rounded-lg border border-border text-foreground hover:bg-muted transition-colors text-sm font-medium"
            >
              取消
            </button>
            <button
              @click="handleConfirm"
              :class="[
                'flex-1 px-4 py-2 rounded-lg text-white text-sm font-medium transition-colors',
                confirmType === 'danger' ? 'bg-destructive hover:brightness-110' : 'bg-primary hover:brightness-110'
              ]"
            >
              {{ confirmText }}
            </button>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
const props = withDefaults(defineProps<{
  modelValue: boolean
  title?: string
  message?: string
  confirmText?: string
  confirmType?: 'primary' | 'danger'
}>(), {
  title: '确认',
  message: '确定要执行此操作吗？',
  confirmText: '确认',
  confirmType: 'primary',
})

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  'confirm': []
  'cancel': []
}>()

function handleConfirm() {
  emit('update:modelValue', false)
  emit('confirm')
}

function handleCancel() {
  emit('update:modelValue', false)
  emit('cancel')
}
</script>

<style scoped>
.modal-enter-active,
.modal-leave-active {
  transition: all 0.3s ease;
}
.modal-enter-from,
.modal-leave-to {
  opacity: 0;
}
</style>
