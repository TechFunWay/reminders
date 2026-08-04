<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center" @click.self="handleClose">
        <div class="absolute inset-0 bg-black bg-opacity-50"></div>
        <div class="relative bg-surface text-foreground rounded-2xl shadow-xl p-6 w-full max-w-lg mx-4 max-h-[90vh] overflow-auto">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-bold text-foreground">{{ title }}</h3>
            <button @click="handleClose" class="text-muted-foreground hover:text-foreground transition-colors">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          <div>
            <slot />
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
const props = defineProps<{
  modelValue: boolean
  title?: string
  closable?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

function handleClose() {
  if (props.closable !== false) {
    emit('update:modelValue', false)
  }
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
.modal-enter-from .relative,
.modal-leave-to .relative {
  transform: scale(0.95);
}
</style>
