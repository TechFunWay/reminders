<template>
  <button
    type="button"
    class="ui-toggle"
    @click="themeStore.toggle()"
    :title="tooltip"
    :aria-label="tooltip"
  >
    <svg v-if="themeStore.mode === 'dark'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
    </svg>
    <svg v-else-if="themeStore.mode === 'light'" class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
    </svg>
    <svg v-else class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24" aria-hidden="true">
      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 17.25v1.007a3 3 0 01-.879 2.122L7.5 21h9l-.621-.621A3 3 0 0115 18.257V17.25m6-12V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15V5.25m18 0A2.25 2.25 0 0018.75 3H5.25A2.25 2.25 0 003 5.25m18 0V15a2.25 2.25 0 01-2.25 2.25H5.25A2.25 2.25 0 013 15" />
    </svg>
  </button>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useThemeStore } from '../../stores/theme'

const themeStore = useThemeStore()
const tooltip = computed(() => ({
  light: '浅色（点击切到跟随系统）',
  dark:  '深色（点击切到浅色）',
  system: '跟随系统（点击切到深色）',
}[themeStore.mode]))
</script>

<style scoped>
.ui-toggle {
  @apply p-2.5 rounded-xl text-foreground hover:bg-muted transition-colors;
}
</style>