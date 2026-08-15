import type { SystemBranding } from '~/types/api'

export const DEFAULT_BRANDING: SystemBranding = {
  app_name: 'ShortURL', organization_name: '', tagline: 'Simple links. Clear insights.',
  login_heading: 'Make every link easier to share and understand.',
  login_description: 'Create branded short links, understand your audience, and keep your whole team working in one place.',
  footer_text: 'Secure, self-hosted, and built for your team.', support_email: '', support_url: '',
  documentation_url: '', privacy_url: '', terms_url: '', primary_color: '#16a34a',
  shell_color: '#172033', show_powered_by: true, logo_light_url: '', logo_dark_url: '',
  logo_compact_url: '', favicon_url: '',
  qr_foreground: '#172033', qr_background: '#ffffff', qr_style: 'rounded', qr_corner_style: 'rounded',
  qr_margin: 2, qr_size: 1024, qr_use_logo: true,
}

export function useBranding() {
  const branding = useState<SystemBranding>('system-branding', () => ({ ...DEFAULT_BRANDING }))
  const loaded = useState('system-branding-loaded', () => false)
  const activeCustomTheme = useState<string | null>('active-custom-theme', () => null)
  const colorMode = useColorMode()

  function applyColors() {
    if (!import.meta.client || activeCustomTheme.value) return
    const root = document.documentElement
    const primary = branding.value.primary_color
    root.style.setProperty('--color-accent', primary)
    root.style.setProperty('--color-accent-hover', darken(primary, 0.8))
    root.style.setProperty('--color-accent-content', contrastColor(primary))
    root.style.setProperty('--color-shell-accent', primary)
    root.style.setProperty('--color-shell', colorMode.value === 'dark' ? darken(branding.value.shell_color) : branding.value.shell_color)
  }

  async function load(force = false) {
    if (loaded.value && !force) return branding.value
    try {
      branding.value = await useServices().branding.get()
    } catch {
      branding.value = { ...DEFAULT_BRANDING }
    }
    loaded.value = true
    applyColors()
    return branding.value
  }

  function set(value: SystemBranding) {
    branding.value = value
    loaded.value = true
    applyColors()
  }

  watch([branding, () => colorMode.value, activeCustomTheme], () => {
    if (!activeCustomTheme.value) applyColors()
  }, { deep: true })
  onMounted(() => load())

  useHead(() => ({
    titleTemplate: title => title ? title.replaceAll('ShortURL', branding.value.app_name) : branding.value.app_name,
    meta: [{ name: 'application-name', content: branding.value.app_name }],
    link: branding.value.favicon_url ? [{ rel: 'icon', href: branding.value.favicon_url }] : [],
  }))

  return { branding, loaded, load, set }
}

function darken(hex: string, factor = 0.58): string {
  if (!/^#[0-9a-f]{6}$/i.test(hex)) return hex
  const values = [1, 3, 5].map(index => Math.round(Number.parseInt(hex.slice(index, index + 2), 16) * factor))
  return `#${values.map(value => value.toString(16).padStart(2, '0')).join('')}`
}

function contrastColor(hex: string): string {
  if (!/^#[0-9a-f]{6}$/i.test(hex)) return '#ffffff'
  const red = Number.parseInt(hex.slice(1, 3), 16)
  const green = Number.parseInt(hex.slice(3, 5), 16)
  const blue = Number.parseInt(hex.slice(5, 7), 16)
  const luminance = (0.299 * red + 0.587 * green + 0.114 * blue) / 255
  return luminance > 0.6 ? '#172033' : '#ffffff'
}
