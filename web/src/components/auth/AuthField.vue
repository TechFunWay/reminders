<template>
  <div>
    <div v-if="label || $slots.labelRight" class="flex items-center justify-between mb-1.5">
      <label v-if="label" class="block text-sm font-medium text-white/75">{{ label }}</label>
      <slot name="labelRight" />
    </div>
    <div class="relative group">
      <span
        v-if="$slots.icon"
        class="absolute inset-y-0 left-0 pl-3.5 flex items-center text-white/35 group-focus-within:text-brand-300 transition-colors"
      >
        <slot name="icon" />
      </span>
      <input
        :value="modelValue"
        @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
        :type="inputType"
        :required="required"
        :autocomplete="autocomplete"
        :placeholder="placeholder"
        :autofocus="autofocus || undefined"
        :class="[
          'w-full py-3 rounded-xl bg-white/[0.04] border border-white/10 text-white placeholder-white/30 focus:bg-white/[0.07] focus:border-brand-400/70 focus:ring-4 focus:ring-brand-500/20 outline-none transition-all',
          $slots.icon ? 'pl-11' : 'pl-4',
          isPassword ? 'pr-11' : 'pr-4',
        ]"
      />
      <button
        v-if="isPassword"
        type="button"
        tabindex="-1"
        @click="show = !show"
        class="absolute inset-y-0 right-0 pr-3.5 flex items-center text-white/35 hover:text-white/70 transition-colors"
      >
        <svg v-if="show" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 12a3 3 0 11-6 0 3 3 0 016 0z"/><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z"/></svg>
        <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.875 18.825A10.05 10.05 0 0112 19c-4.478 0-8.268-2.943-9.543-7a9.97 9.97 0 011.563-3.029m5.858.908a3 3 0 114.243 4.243M9.878 9.878l4.242 4.242M9.88 9.88l-3.29-3.29m7.532 7.532l3.29 3.29M3 3l3.59 3.59m0 0A9.953 9.953 0 0112 5c4.478 0 8.268 2.943 9.543 7a10.025 10.025 0 01-4.132 5.411m0 0L21 21"/></svg>
      </button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'

const props = withDefaults(
  defineProps<{
    modelValue: string
    label?: string
    type?: string
    placeholder?: string
    autocomplete?: string
    required?: boolean
    autofocus?: boolean
  }>(),
  { type: 'text', required: false },
)

defineEmits<{ (e: 'update:modelValue', value: string): void }>()

const show = ref(false)
const isPassword = computed(() => props.type === 'password')
const inputType = computed(() => (isPassword.value ? (show.value ? 'text' : 'password') : props.type))
</script>
