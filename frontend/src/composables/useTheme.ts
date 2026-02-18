import { ref, watch } from 'vue'

const STORAGE_KEY = 'app_theme'

function getSystemDark(): boolean {
  return window.matchMedia('(prefers-color-scheme: dark)').matches
}

function getInitialDark(): boolean {
  const stored = localStorage.getItem(STORAGE_KEY)
  if (stored === 'dark') return true
  if (stored === 'light') return false
  return getSystemDark()
}

const isDark = ref(getInitialDark())

function applyTheme(dark: boolean) {
  document.documentElement.classList.toggle('dark', dark)
  localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
}

export function useTheme() {
  // Apply on first use
  applyTheme(isDark.value)

  watch(isDark, (v) => applyTheme(v))

  function toggle() {
    isDark.value = !isDark.value
  }

  return { isDark, toggle }
}
