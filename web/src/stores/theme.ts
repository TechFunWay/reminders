import {
  defineStore
} from 'pinia'
import {
  ref,
  computed
} from 'vue'

export type ThemeMode = 'light' | 'dark' | 'system'
export type ThemeBrand = 'default'  // extend this union when adding brand-* palettes

const STORAGE_MODE = 'reminder-theme-mode'
const STORAGE_BRAND = 'reminder-theme-brand'

function readStoredMode(): ThemeMode {
  const v = localStorage.getItem(STORAGE_MODE)
  return v === 'light' || v === 'dark' || v === 'system' ? v : 'system'
}

function readStoredBrand(): ThemeBrand {
  const v = localStorage.getItem(STORAGE_BRAND)
  return (v as ThemeBrand) || 'default'
}

const systemDark = ref(
  typeof window !== 'undefined'
    && window.matchMedia('(prefers-color-scheme: dark)').matches,
)

if (typeof window !== 'undefined') {
  window.matchMedia('(prefers-color-scheme: dark)')
    .addEventListener('change', e => { systemDark.value = e.matches })
}

export const useThemeStore = defineStore('theme', () => {
  const mode = ref<ThemeMode>(readStoredMode())
  const brand = ref<ThemeBrand>(readStoredBrand())

  // isDark tracks mode AND the OS preference (via systemDark ref) so
  // consumers bound to it update when the OS theme flips in 'system'.
  const isDark = computed(() =>
    mode.value === 'dark' || (mode.value === 'system' && systemDark.value),
  )

  function resolveDark(): boolean {
    if (mode.value === 'dark') return true
    if (mode.value === 'light') return false
    return systemDark.value
  }

  function apply() {
    const root = document.documentElement
    root.classList.toggle('dark', resolveDark())

    for (const cls of Array.from(root.classList)) {
      if (cls.startsWith('brand-') && cls !== `brand-${brand.value}`) {
        root.classList.remove(cls)
      }
    }
    if (brand.value !== 'default') {
      root.classList.add(`brand-${brand.value}`)
    }
  }

  function setMode(m: ThemeMode) {
    mode.value = m
    localStorage.setItem(STORAGE_MODE, m)
    apply()
  }

  function setBrand(b: ThemeBrand) {
    brand.value = b
    localStorage.setItem(STORAGE_BRAND, b)
    apply()
  }

  // 3-state cycle documented in AGENTS.md. Components should call this so
  // the cycle stays in one place.
  function toggle() {
    const next: ThemeMode = mode.value === 'dark'
      ? 'light'
      : mode.value === 'light'
        ? 'system'
        : 'dark'
    setMode(next)
  }

  apply()

  return { mode, brand, isDark, toggle, setMode, setBrand, apply }
})
