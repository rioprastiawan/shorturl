export type Language = 'en' | 'id'

const messages = {
  en: {
    workspace: 'Workspace', manage: 'Manage', developer: 'Developer', system: 'System',
    overview: 'Overview', links: 'Links', analytics: 'Analytics', domains: 'Domains',
    members: 'Team members', api: 'API keys', apiDocs: 'API documentation', settings: 'Settings', profile: 'Profile',
    workspaceSettings: 'Workspace settings', accountSettings: 'Account settings',
    currentWorkspace: 'Current workspace', appearance: 'Appearance', signOut: 'Sign out',
    light: 'Light', dark: 'Dark', systemMode: 'System',
    preferences: 'Language & timezone', preferencesDescription: 'Choose the language and timezone used by your account.',
    language: 'Language', timezone: 'Timezone', english: 'English', indonesian: 'Bahasa Indonesia',
    savePreferences: 'Save preferences', preferencesSaved: 'Preferences saved',
  },
  id: {
    workspace: 'Ruang kerja', manage: 'Kelola', developer: 'Pengembang', system: 'Sistem',
    overview: 'Ringkasan', links: 'Tautan', analytics: 'Analitik', domains: 'Domain',
    members: 'Anggota tim', api: 'API key', apiDocs: 'Dokumentasi API', settings: 'Pengaturan', profile: 'Profil',
    workspaceSettings: 'Pengaturan workspace', accountSettings: 'Pengaturan akun',
    currentWorkspace: 'Ruang kerja aktif', appearance: 'Tampilan', signOut: 'Keluar',
    light: 'Terang', dark: 'Gelap', systemMode: 'Sistem',
    preferences: 'Bahasa & zona waktu', preferencesDescription: 'Pilih bahasa dan zona waktu yang digunakan akun Anda.',
    language: 'Bahasa', timezone: 'Zona waktu', english: 'English', indonesian: 'Bahasa Indonesia',
    savePreferences: 'Simpan preferensi', preferencesSaved: 'Preferensi tersimpan',
  },
} as const

type MessageKey = keyof typeof messages.en

export function useUserPreferences() {
  const session = useSession()
  const language = computed<Language>(() => session.user.value?.language || 'en')
  const timezone = computed(() => session.user.value?.timezone || 'UTC')
  const t = (key: MessageKey) => messages[language.value][key]
  return { language, timezone, t }
}
