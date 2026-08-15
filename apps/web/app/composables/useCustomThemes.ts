export const THEME_TOKENS = [
  { key: 'surface', label: 'Page background' },
  { key: 'surface-muted', label: 'Muted surface' },
  { key: 'surface-raised', label: 'Cards & panels' },
  { key: 'border', label: 'Border' },
  { key: 'border-strong', label: 'Strong border' },
  { key: 'content', label: 'Primary text' },
  { key: 'content-muted', label: 'Muted text' },
  { key: 'content-subtle', label: 'Subtle text' },
  { key: 'accent', label: 'Accent' },
  { key: 'accent-hover', label: 'Accent hover' },
  { key: 'accent-content', label: 'Text on accent' },
  { key: 'danger', label: 'Danger' },
  { key: 'danger-hover', label: 'Danger hover' },
  { key: 'success', label: 'Success' },
  { key: 'warning', label: 'Warning' },
  { key: 'shell', label: 'Navigation shell' },
  { key: 'shell-content', label: 'Shell text' },
  { key: 'shell-accent', label: 'Shell accent' },
] as const

export type ThemeToken = typeof THEME_TOKENS[number]['key']
export type ThemePalette = Record<ThemeToken, string>

export interface CustomTheme {
  id: string
  name: string
  light: ThemePalette
  dark: ThemePalette
  updatedAt: string
}

export const DEFAULT_LIGHT_PALETTE: ThemePalette = {
  surface: '#f8fafc', 'surface-muted': '#f1f5f9', 'surface-raised': '#ffffff',
  border: '#e2e8f0', 'border-strong': '#cbd5e1', content: '#172033',
  'content-muted': '#64748b', 'content-subtle': '#94a3b8', accent: '#16a34a',
  'accent-hover': '#15803d', 'accent-content': '#ffffff', danger: '#dc2626',
  'danger-hover': '#b91c1c', success: '#059669', warning: '#d97706',
  shell: '#172033', 'shell-content': '#ffffff', 'shell-accent': '#16a34a',
}

export const DEFAULT_DARK_PALETTE: ThemePalette = {
  surface: '#0f141b', 'surface-muted': '#0b1016', 'surface-raised': '#171e28',
  border: '#293341', 'border-strong': '#3a4657', content: '#f1f5f9',
  'content-muted': '#a3adbd', 'content-subtle': '#6f7b8d', accent: '#4ade80',
  'accent-hover': '#6ee7a0', 'accent-content': '#07150d', danger: '#f87171',
  'danger-hover': '#fca5a5', success: '#34d399', warning: '#fbbf24',
  shell: '#080c12', 'shell-content': '#ffffff', 'shell-accent': '#4ade80',
}

const STORAGE_KEY = 'shorturl.custom-themes.v1'
const ACTIVE_KEY = 'shorturl.active-custom-theme.v1'

export function useCustomThemes() {
  const colorMode = useColorMode()
  const themes = useState<CustomTheme[]>('custom-themes', () => [])
  const activeId = useState<string | null>('active-custom-theme', () => null)
  const initialized = useState('custom-themes-initialized', () => false)
  const activeTheme = computed(() => themes.value.find(theme => theme.id === activeId.value) ?? null)

  function apply() {
    if (!import.meta.client) return
    const root = document.documentElement
    const palette = activeTheme.value?.[colorMode.value === 'dark' ? 'dark' : 'light']
    for (const token of THEME_TOKENS) {
      const property = `--color-${token.key}`
      if (palette) root.style.setProperty(property, palette[token.key])
      else root.style.removeProperty(property)
    }
  }

  function persist() {
    if (!import.meta.client) return
    localStorage.setItem(STORAGE_KEY, JSON.stringify(themes.value))
    if (activeId.value) localStorage.setItem(ACTIVE_KEY, activeId.value)
    else localStorage.removeItem(ACTIVE_KEY)
  }

  function initialize() {
    if (!import.meta.client || initialized.value) return
    initialized.value = true
    try {
      const stored = JSON.parse(localStorage.getItem(STORAGE_KEY) ?? '[]')
      themes.value = Array.isArray(stored) ? stored : []
      activeId.value = localStorage.getItem(ACTIVE_KEY)
      if (!themes.value.some(theme => theme.id === activeId.value)) activeId.value = null
    } catch {
      themes.value = []
      activeId.value = null
    }
    apply()
  }

  function save(theme: CustomTheme) {
    const index = themes.value.findIndex(item => item.id === theme.id)
    if (index >= 0) themes.value[index] = theme
    else themes.value.push(theme)
    activeId.value = theme.id
    persist()
    apply()
  }

  function activate(id: string | null) {
    activeId.value = id
    persist()
    apply()
  }

  function remove(id: string) {
    themes.value = themes.value.filter(theme => theme.id !== id)
    if (activeId.value === id) activeId.value = null
    persist()
    apply()
  }

  watch([() => colorMode.value, activeTheme], apply, { deep: true })
  onMounted(initialize)

  return { themes, activeId, activeTheme, initialize, save, activate, remove }
}
