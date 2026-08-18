/**
 * Reactive translation bridge for legacy dashboard copy.
 *
 * New code should prefer `tr(english, indonesian)` directly. The dashboard
 * predates account languages, however, and still contains a large amount of
 * plain template copy. This bridge keeps those text nodes and accessibility
 * attributes bilingual while they are migrated, including content mounted in
 * native dialogs, teleports, and dropdown popovers.
 */
export default defineNuxtPlugin(() => {
  const { language } = useUserPreferences()
  type TranslationState = { original: string, applied: string }
  const textStates = new WeakMap<Text, TranslationState>()
  const attributeStates = new WeakMap<Element, Map<string, TranslationState>>()
  const attributes = ['placeholder', 'title', 'aria-label'] as const

  const id: Record<string, string> = {
    'Account Settings': 'Pengaturan Akun', 'Activity log': 'Log aktivitas', 'Add tags': 'Tambah tag',
    'Appearance': 'Tampilan', 'Apply': 'Terapkan', 'Apply range': 'Terapkan rentang', 'Archive': 'Arsipkan',
    'Authentication': 'Autentikasi', 'Average clicks/day': 'Rata-rata klik/hari', 'Base URL': 'URL dasar',
    'Brand': 'Merek', 'Cancel': 'Batal', 'Change password': 'Ubah kata sandi', 'Checking': 'Memeriksa',
    'Classic': 'Klasik', 'Clear': 'Bersihkan', 'Clicks in period': 'Klik dalam periode', 'Close': 'Tutup',
    'Connect your domain': 'Hubungkan domain Anda', 'Connection': 'Koneksi', 'Copy': 'Salin',
    'Create': 'Buat', 'Create demo': 'Buat demo', 'Create link': 'Buat tautan',
    'Create your first link': 'Buat tautan pertama', 'Created': 'Dibuat', 'Custom': 'Kustom',
    'Custom date range': 'Rentang tanggal kustom', 'Custom themes': 'Tema kustom', 'Dashboard': 'Dasbor',
    'Date range': 'Rentang tanggal', 'Delete': 'Hapus', 'Delete workspace': 'Hapus workspace',
    'Destination': 'Tujuan', 'Developer': 'Pengembang', 'Disable': 'Nonaktifkan',
    'Disable verification': 'Nonaktifkan verifikasi', 'Done': 'Selesai', 'Download template': 'Unduh template',
    'Edit': 'Edit', 'Enable': 'Aktifkan', 'Example request body': 'Contoh body request',
    'Example success response': 'Contoh respons berhasil', 'Failed': 'Gagal', 'Home': 'Beranda',
    'Import preview': 'Pratinjau impor', 'Initial setup': 'Pengaturan awal', 'Internal notes': 'Catatan internal',
    'Inverse': 'Terbalik', 'Invitation link': 'Tautan undangan', 'Keyboard shortcuts': 'Pintasan keyboard',
    'Language': 'Bahasa', 'Line': 'Baris', 'Links': 'Tautan', 'Login description': 'Deskripsi login',
    'Manage': 'Kelola', 'Manual setup key': 'Kunci pengaturan manual', 'More': 'Lainnya',
    'New custom theme': 'Tema kustom baru', 'Next': 'Berikutnya', 'No matching commands': 'Tidak ada perintah yang cocok',
    'Overview': 'Ringkasan', 'Personalization': 'Personalisasi', 'Possible responses': 'Kemungkinan respons',
    'Previous': 'Sebelumnya', 'Public registration': 'Pendaftaran publik', 'QR Branding': 'Branding QR',
    'Recovery codes': 'Kode pemulihan', 'Remove': 'Hapus', 'Reset defaults': 'Reset default',
    'Reset filters': 'Reset filter', 'Reset whitelabeling': 'Reset whitelabeling', 'Response format': 'Format respons',
    'Retry': 'Coba lagi', 'Revoke': 'Cabut', 'Rows': 'Baris', 'Rows requiring attention': 'Baris yang perlu diperiksa',
    'Save & apply': 'Simpan & terapkan', 'Save QR branding': 'Simpan branding QR',
    'Save account details': 'Simpan detail akun', 'Save changes': 'Simpan perubahan',
    'Save redirect': 'Simpan pengalihan', 'Save whitelabeling': 'Simpan whitelabeling',
    'Set up': 'Atur', 'Short URL QR code': 'Kode QR URL pendek', 'Sign in': 'Masuk', 'Sign out': 'Keluar',
    'Slug / domain': 'Slug / domain', 'Status': 'Status', 'System': 'Sistem', 'System Settings': 'Pengaturan Sistem',
    'Today': 'Hari ini', 'Try again': 'Coba lagi', 'Up to date': 'Sudah terbaru',
    'Update available': 'Pembaruan tersedia', 'Update password': 'Perbarui kata sandi', 'Use default': 'Gunakan default',
    'Verification needs attention': 'Verifikasi perlu diperiksa', 'Welcome back': 'Selamat datang kembali',
    'Within the next 7 days': 'Dalam 7 hari ke depan', 'Workspace': 'Workspace',
    'Workspace Settings': 'Pengaturan Workspace', 'Your role': 'Peran Anda',
    'A chronological record of link changes in this workspace.': 'Catatan kronologis perubahan di workspace ini.',
    'Choose any period up to 366 days. Times use your local timezone.': 'Pilih periode hingga 366 hari. Waktu menggunakan zona waktu lokal Anda.',
    'Customize the default appearance and brand identity of every QR code.': 'Sesuaikan tampilan default dan identitas merek setiap kode QR.',
    'Customize this entire installation for your organization.': 'Sesuaikan seluruh instalasi ini untuk organisasi Anda.',
    'Each theme contains a Light and Dark palette and stays in this browser.': 'Setiap tema memiliki palet Terang dan Gelap serta tersimpan di browser ini.',
    'Enter a complete six-digit hex color.': 'Masukkan warna hex enam digit yang lengkap.',
    'Installation profile is unavailable.': 'Profil instalasi tidak tersedia.',
    'Manage this ShortURL installation and check for updates.': 'Kelola instalasi ShortURL ini dan periksa pembaruan.',
    'Manage your profile, preferences, appearance, and password.': 'Kelola profil, preferensi, tampilan, dan kata sandi Anda.',
    'Need the complete API reference?': 'Perlu referensi API lengkap?',
    'Need the correct format?': 'Perlu format yang benar?',
    'Only visible to workspace members.': 'Hanya terlihat oleh anggota workspace.',
    'Place the Whitelabeling logo in the center': 'Tempatkan logo Whitelabeling di tengah',
    'Press Enter to run the first result': 'Tekan Enter untuk menjalankan hasil pertama',
    'Scan with your authenticator app': 'Pindai dengan aplikasi autentikator Anda',
    'Scan, copy, or download using the system appearance.': 'Pindai, salin, atau unduh menggunakan tampilan sistem.',
    'Sign in to continue.': 'Masuk untuk melanjutkan.',
    'Store these somewhere safe. Each code works once.': 'Simpan di tempat aman. Setiap kode hanya dapat digunakan sekali.',
    'System follows your device. A custom theme automatically switches to its matching palette.': 'Mode sistem mengikuti perangkat Anda. Tema kustom otomatis memakai palet yang sesuai.',
    'This link can be used once and expires after 7 days.': 'Tautan ini hanya dapat digunakan sekali dan kedaluwarsa setelah 7 hari.',
    'Uses the selected role; no email is required.': 'Menggunakan peran yang dipilih; email tidak diperlukan.',
    'Visitors will be sent to the ShortURL dashboard, matching the default behavior.': 'Pengunjung akan diarahkan ke dasbor ShortURL sesuai perilaku default.',
    'Changes are previewed instantly and apply globally after saving.': 'Perubahan langsung dipratinjau dan diterapkan secara global setelah disimpan.',
    'Could not reach GitHub Releases. Try again later.': 'Tidak dapat menghubungi GitHub Releases. Coba lagi nanti.',
    'Download a ready-to-edit CSV with every supported column.': 'Unduh CSV siap edit dengan semua kolom yang didukung.',
    'Demo short URLs use an example domain. Replace it with a verified domain before sharing publicly.': 'URL pendek demo menggunakan domain contoh. Ganti dengan domain terverifikasi sebelum dibagikan ke publik.',
    'Permanently removes its links, domains, members, and click history. Short links stop resolving immediately.': 'Menghapus tautan, domain, anggota, dan riwayat klik secara permanen. Tautan pendek akan langsung berhenti berfungsi.',
    'Personal browser-only themes. These settings do not affect teammates or system-wide whitelabeling.': 'Tema pribadi yang hanya tersimpan di browser. Pengaturan ini tidak memengaruhi rekan tim atau whitelabeling sistem.',
    'Preview only. Click “Save whitelabeling” to apply these changes globally.': 'Hanya pratinjau. Klik “Simpan whitelabeling” untuk menerapkan perubahan secara global.',
    'Trend, totals and timing are filtered. Audience breakdowns are hidden because those rollups are workspace-wide.': 'Tren, total, dan waktu telah difilter. Rincian audiens disembunyikan karena rangkumannya mencakup seluruh workspace.',
    'Update your password and sign out other sessions.': 'Perbarui kata sandi dan keluarkan sesi lainnya.',
    'Updates are not installed automatically. Review the release notes, back up persistent data, then deploy the matching image tag in Dokploy or update the checked-out source.': 'Pembaruan tidak dipasang otomatis. Tinjau catatan rilis, cadangkan data persisten, lalu deploy image tag yang sesuai di Dokploy atau perbarui source.',
    'Upload a compact or light logo in Whitelabeling to enable this option.': 'Unggah logo ringkas atau terang di Whitelabeling untuk mengaktifkan opsi ini.',
    'Your link performance at a glance.': 'Ringkasan performa tautan Anda.',
    'or drag and drop here · maximum 1 MB and 1,000 links': 'atau tarik dan lepas di sini · maksimal 1 MB dan 1.000 tautan',
  }

  function translated(value: string) {
    const leading = value.match(/^\s*/)?.[0] ?? ''
    const trailing = value.match(/\s*$/)?.[0] ?? ''
    const clean = value.trim()
    if (!clean) return value
    let result = id[clean]
    if (!result) {
      result = clean
        .replace(/^Welcome back,\s*/i, 'Selamat datang kembali, ')
        .replace(/^Showing (.+) links$/, 'Menampilkan $1 tautan')
        .replace(/^Showing (.+) domains$/, 'Menampilkan $1 domain')
        .replace(/^Showing (.+) members$/, 'Menampilkan $1 anggota')
        .replace(/^Page (\d+)$/, 'Halaman $1')
    }
    return leading + result + trailing
  }

  function skip(node: Node) {
    const parent = node instanceof Element ? node : node.parentElement
    return Boolean(parent?.closest('pre, code, script, style, [data-no-translate]'))
  }

  function visit(root: Node) {
    if (skip(root)) return
    if (root.nodeType === Node.TEXT_NODE) {
      const text = root as Text
      const current = text.textContent ?? ''
      let state = textStates.get(text)
      if (!state) state = { original: current, applied: current }
      else if (current !== state.applied) state.original = current
      const next = language.value === 'id' ? translated(state.original) : state.original
      state.applied = next
      textStates.set(text, state)
      if (current !== next) text.textContent = next
      return
    }
    if (root instanceof Element) {
      let stored = attributeStates.get(root)
      if (!stored) {
        stored = new Map<string, TranslationState>()
        attributeStates.set(root, stored)
      }
      for (const attribute of attributes) {
        const current = root.getAttribute(attribute)
        if (!current) continue
        let state = stored.get(attribute)
        if (!state) state = { original: current, applied: current }
        else if (current !== state.applied) state.original = current
        const next = language.value === 'id' ? translated(state.original) : state.original
        state.applied = next
        stored.set(attribute, state)
        if (current !== next) root.setAttribute(attribute, next)
      }
    }
    for (const child of root.childNodes) visit(child)
  }

  let translating = false
  function translatePage() {
    if (translating || !location.pathname.startsWith('/dashboard')) return
    translating = true
    visit(document.body)
    queueMicrotask(() => { translating = false })
  }

  const observer = new MutationObserver(translatePage)
  onNuxtReady(() => {
    translatePage()
    observer.observe(document.body, { childList: true, subtree: true, characterData: true })
  })
  watch(language, () => nextTick(translatePage))
})
