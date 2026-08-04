<template>
  <Teleport to="body">
    <Transition name="modal">
      <div v-if="modelValue" class="fixed inset-0 z-50 flex items-center justify-center" @click.self="handleClose">
        <div class="absolute inset-0 bg-black bg-opacity-50"></div>
        <div class="relative bg-surface text-foreground rounded-2xl shadow-xl p-6 w-full max-w-sm mx-4">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-bold text-foreground">二维码</h3>
            <button @click="handleClose" class="text-muted-foreground hover:text-foreground transition-colors">
              <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12" /></svg>
            </button>
          </div>
          <div class="flex justify-center">
            <img
              v-if="qrUrl"
              :src="qrUrl"
              alt="QR Code"
              class="w-64 h-64 rounded-lg"
            />
            <div v-else class="w-64 h-64 flex items-center justify-center bg-muted rounded-lg">
              <span class="text-sm text-muted-foreground">加载中...</span>
            </div>
          </div>
          <div v-if="content" class="mt-4 text-center text-xs text-muted-foreground break-all">{{ content }}</div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'

const props = defineProps<{
  modelValue: boolean
  content: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
}>()

const qrUrl = ref('')

watch(() => props.modelValue, async (val) => {
  if (val && props.content) {
    qrUrl.value = `/api/qrcode?content=${encodeURIComponent(props.content)}&t=${Date.now()}`
  } else {
    qrUrl.value = ''
  }
})

function handleClose() {
  emit('update:modelValue', false)
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
