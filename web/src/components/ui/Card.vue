<template>
  <div :class="['ui-card', { 'ui-card--bordered': bordered }, { 'ui-card--interactive': interactive }]">
    <header v-if="hasHeader" class="ui-card__header">
      <div class="ui-card__heading">
        <slot name="header">
          <h3 v-if="title" class="ui-card__title">{{ title }}</h3>
          <p v-if="subtitle" class="ui-card__subtitle">{{ subtitle }}</p>
        </slot>
      </div>
      <div v-if="$slots.actions" class="ui-card__actions">
        <slot name="actions" />
      </div>
    </header>
    <div :class="['ui-card__body', { 'ui-card__body--flush': hasHeader }]">
      <slot />
    </div>
    <footer v-if="$slots.footer" class="ui-card__footer">
      <slot name="footer" />
    </footer>
  </div>
</template>

<script setup lang="ts">
import { computed, useSlots } from 'vue'

const props = withDefaults(
  defineProps<{
    title?: string
    subtitle?: string
    bordered?: boolean
    interactive?: boolean
  }>(),
  { bordered: true, interactive: false },
)

const slots = useSlots()
// Body top padding is dropped only when a header is rendered above it so
// headerless cards still get full interior padding on all four sides.
const hasHeader = computed(() => !!(props.title || slots.header || slots.actions))
</script>

<style scoped>
.ui-card {
  @apply rounded-2xl bg-surface text-surface-foreground shadow-card transition-all;
}
.ui-card--bordered { @apply border border-border; }
.ui-card--interactive:hover { @apply -translate-y-0.5 shadow-soft; }

.ui-card__header {
  @apply flex items-start justify-between gap-3 p-5 pb-3;
}
.ui-card__title    { @apply text-base font-display font-bold; }
.ui-card__subtitle { @apply text-xs text-muted-foreground mt-0.5; }
.ui-card__actions  { @apply shrink-0; }

.ui-card__body            { @apply p-5; }
.ui-card__body--flush     { @apply pt-0; }
.ui-card__footer          { @apply p-5 pt-4 border-t border-border; }
</style>