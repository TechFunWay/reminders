<template>
  <button
    :type="type"
    :disabled="disabled"
    :class="['ui-btn', `ui-btn--${variant}`, `ui-btn--${size}`, { 'ui-btn--block': block }, { 'ui-btn--icon-only': !$slots.default }]"
  >
    <slot name="icon-left" />
    <slot />
    <slot name="icon-right" />
  </button>
</template>

<script setup lang="ts">
import { type ButtonHTMLAttributes } from 'vue'

withDefaults(
  defineProps<{
    type?: ButtonHTMLAttributes['type']
    variant?: 'primary' | 'ghost' | 'soft' | 'danger'
    size?: 'sm' | 'md' | 'lg'
    block?: boolean
    disabled?: boolean
  }>(),
  { type: 'button', variant: 'primary', size: 'md', block: false, disabled: false },
)
</script>

<style scoped>
.ui-btn {
  @apply inline-flex items-center justify-center gap-2 font-medium rounded-xl outline-none transition-all active:scale-[0.98] disabled:opacity-50 disabled:pointer-events-none;
}
.ui-btn--sm { @apply text-xs px-3 py-1.5; }
.ui-btn--md { @apply text-sm px-4 py-2.5; }
.ui-btn--lg { @apply text-base px-5 py-3; }
.ui-btn--block { @apply w-full; }
.ui-btn--icon-only { @apply px-2.5; }

.ui-btn--primary {
  @apply bg-brand-gradient text-white shadow-glow hover:brightness-110;
}
.ui-btn--ghost {
  @apply bg-transparent text-foreground border border-border hover:bg-muted;
}
.ui-btn--soft {
  @apply bg-muted text-foreground hover:brightness-95;
}
.ui-btn--danger {
  @apply bg-destructive text-white hover:brightness-110;
}
</style>