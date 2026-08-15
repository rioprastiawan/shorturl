<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'
import QRCode from 'qrcode'
import type { TwoFactorSetup } from '~/types/api'

definePageMeta({ middleware: 'auth' })
useHead({ title: 'Account Settings · ShortURL' })

const session = useSession()
const { auth } = useServices()
const toast = useToast()
const { t } = useUserPreferences()

const preferenceLanguage = ref<'en' | 'id'>(session.user.value?.language || 'en')
const preferenceTimezone = ref(session.user.value?.timezone || 'UTC')
const preferencesSaving = ref(false)
const preferenceErrors = reactive<{ language?: string, timezone?: string }>({})
const languageOptions = computed(() => [{ value: 'en', label: t('english') }, { value: 'id', label: t('indonesian') }])
const timezoneOptions = computed(() => {
  const supported = typeof Intl.supportedValuesOf === 'function' ? Intl.supportedValuesOf('timeZone') : ['UTC', 'Asia/Jakarta']
  return supported.includes('UTC') ? supported : ['UTC', ...supported]
})
const timezoneSelectOptions = computed(() => timezoneOptions.value.map(zone => ({ value: zone, label: zone.replaceAll('_', ' ') })))
const preferencesDirty = computed(() => preferenceLanguage.value !== (session.user.value?.language || 'en') || preferenceTimezone.value !== (session.user.value?.timezone || 'UTC'))

async function savePreferences() {
  preferencesSaving.value = true
  preferenceErrors.language = undefined
  preferenceErrors.timezone = undefined
  try {
    const user = await auth.updatePreferences({ language: preferenceLanguage.value, timezone: preferenceTimezone.value })
    session.set(user)
    toast.success(t('preferencesSaved'))
  } catch (error) {
    if (error instanceof ApiError) {
      preferenceErrors.language = error.field('language')
      preferenceErrors.timezone = error.field('timezone')
      if (!preferenceErrors.language && !preferenceErrors.timezone) toast.error(error.message)
    } else toast.error('Could not save preferences')
  } finally {
    preferencesSaving.value = false
  }
}

const profileName = ref(session.user.value?.name ?? '')
const profileEmail = ref(session.user.value?.email ?? '')
const profileSaving = ref(false)
const profileErrors = reactive<{ name?: string, email?: string }>({})
const profileDirty = computed(() => profileName.value.trim() !== (session.user.value?.name ?? '') || profileEmail.value.trim().toLowerCase() !== (session.user.value?.email ?? '').toLowerCase())

async function saveProfile() {
  profileErrors.name = undefined
  profileErrors.email = undefined
  profileSaving.value = true
  try {
    const user = await auth.updateProfile({ name: profileName.value.trim(), email: profileEmail.value.trim() })
    session.set(user)
    profileName.value = user.name
    profileEmail.value = user.email
    toast.success('Account details updated')
  } catch (error) {
    if (error instanceof ApiError) {
      profileErrors.name = error.field('name')
      profileErrors.email = error.field('email')
      if (!profileErrors.name && !profileErrors.email) toast.error(error.message)
    } else toast.error('Could not update your account')
  } finally {
    profileSaving.value = false
  }
}

const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const passwordOpen = ref(false)
const passwordSaving = ref(false)
const passwordErrors = reactive<{ current?: string, next?: string, confirm?: string }>({})

function resetPasswordForm() {
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  passwordErrors.current = undefined
  passwordErrors.next = undefined
  passwordErrors.confirm = undefined
}
function openPasswordModal() { resetPasswordForm(); passwordOpen.value = true }
watch(passwordOpen, open => { if (!open && !passwordSaving.value) resetPasswordForm() })

async function changePassword() {
  passwordErrors.current = undefined
  passwordErrors.next = undefined
  passwordErrors.confirm = undefined
  if (newPassword.value !== confirmPassword.value) { passwordErrors.confirm = 'Passwords do not match'; return }
  passwordSaving.value = true
  try {
    await auth.changePassword({ current_password: currentPassword.value, new_password: newPassword.value })
    resetPasswordForm()
    passwordOpen.value = false
    toast.success('Password changed. Other sessions were signed out.')
  } catch (error) {
    if (error instanceof ApiError) {
      passwordErrors.current = error.field('current_password')
      passwordErrors.next = error.field('new_password')
      if (!passwordErrors.current && !passwordErrors.next) toast.error(error.message)
    } else toast.error('Could not change your password')
  } finally { passwordSaving.value = false }
}

const twoFactorEnabled = ref(false)
const twoFactorLoading = ref(true)
const twoFactorOpen = ref(false)
const twoFactorPassword = ref('')
const twoFactorCode = ref('')
const twoFactorSetup = ref<TwoFactorSetup | null>(null)
const twoFactorQr = ref('')
const twoFactorBusy = ref(false)
const twoFactorError = ref<string | undefined>()
const disableTwoFactorOpen = ref(false)
const disablePassword = ref('')
const disableCode = ref('')
const disableError = ref<string | undefined>()

onMounted(async () => {
  try { twoFactorEnabled.value = (await auth.twoFactorStatus()).enabled } finally { twoFactorLoading.value = false }
})

function openTwoFactorSetup() {
  twoFactorPassword.value = ''; twoFactorCode.value = ''; twoFactorSetup.value = null
  twoFactorQr.value = ''; twoFactorError.value = undefined; twoFactorOpen.value = true
}

watch(twoFactorOpen, (open) => {
  if (!open && !twoFactorBusy.value) {
    twoFactorPassword.value = ''; twoFactorCode.value = ''; twoFactorSetup.value = null
    twoFactorQr.value = ''; twoFactorError.value = undefined
  }
})

async function beginTwoFactor() {
  twoFactorBusy.value = true; twoFactorError.value = undefined
  try {
    twoFactorSetup.value = await auth.twoFactorSetup({ password: twoFactorPassword.value })
    twoFactorQr.value = await QRCode.toDataURL(twoFactorSetup.value.uri, { width: 220, margin: 1, errorCorrectionLevel: 'M' })
    twoFactorPassword.value = ''
  } catch (error) {
    twoFactorError.value = error instanceof ApiError ? (error.field('password') ?? error.message) : 'Could not start setup'
  } finally { twoFactorBusy.value = false }
}

async function enableTwoFactor() {
  twoFactorBusy.value = true; twoFactorError.value = undefined
  try {
    await auth.twoFactorEnable({ code: twoFactorCode.value })
    twoFactorEnabled.value = true; twoFactorOpen.value = false
    twoFactorSetup.value = null; twoFactorQr.value = ''; twoFactorCode.value = ''
    toast.success('Two-step verification enabled')
  } catch (error) {
    twoFactorError.value = error instanceof ApiError ? (error.field('code') ?? error.message) : 'Could not verify the code'
  } finally { twoFactorBusy.value = false }
}

function openDisableTwoFactor() {
  disablePassword.value = ''; disableCode.value = ''; disableError.value = undefined; disableTwoFactorOpen.value = true
}

async function disableTwoFactor() {
  twoFactorBusy.value = true; disableError.value = undefined
  try {
    await auth.twoFactorDisable({ password: disablePassword.value, code: disableCode.value })
    twoFactorEnabled.value = false; disableTwoFactorOpen.value = false
    toast.success('Two-step verification disabled')
  } catch (error) {
    disableError.value = error instanceof ApiError ? (error.field('password') ?? error.field('code') ?? error.message) : 'Could not disable two-step verification'
  } finally { twoFactorBusy.value = false }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header><p class="mb-1 text-sm font-semibold text-(--color-accent)">System</p><h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Account Settings</h1><p class="text-sm text-(--color-content-muted)">Manage your profile, preferences, appearance, and password.</p></header>
    <UiCard title="Your account" description="Update the details used to identify and sign in to your account.">
      <form class="flex flex-col gap-4" @submit.prevent="saveProfile"><div class="grid gap-4 sm:grid-cols-2"><UiInput v-model="profileName" label="Display name" required :disabled="profileSaving" :error="profileErrors.name" /><UiInput v-model="profileEmail" label="Email" type="email" required :disabled="profileSaving" :error="profileErrors.email" /></div><div><UiButton type="submit" :loading="profileSaving" :disabled="!profileDirty">Save account details</UiButton></div></form>
      <div class="mt-5 flex flex-wrap items-center justify-between gap-3 border-t border-(--color-border) pt-4"><div class="flex items-center gap-3"><span class="grid size-9 shrink-0 place-items-center rounded-lg bg-(--color-surface-muted) text-(--color-content-muted)"><Icon name="lucide:key-round" size="17" /></span><div><h3 class="text-sm font-semibold">Change password</h3><p class="mt-0.5 text-xs text-(--color-content-muted)">Update your password and sign out other sessions.</p></div></div><UiButton variant="secondary" size="sm" @click="openPasswordModal">Change password</UiButton></div>
      <div class="mt-6 border-t border-(--color-border) pt-6"><UiButton variant="ghost" @click="session.logout()"><Icon name="lucide:log-out" size="16" /> Sign out</UiButton></div>
    </UiCard>
    <UiCard title="Two-step verification" description="Add an authenticator code after your password when signing in.">
      <div class="flex flex-wrap items-center justify-between gap-4"><div class="flex items-center gap-3"><span class="grid size-10 place-items-center rounded-xl" :class="twoFactorEnabled ? 'bg-emerald-500/10 text-(--color-success)' : 'bg-(--color-surface-muted) text-(--color-content-muted)'"><Icon name="lucide:shield-check" size="19" /></span><div><p class="text-sm font-semibold">{{ twoFactorLoading ? 'Checking status…' : twoFactorEnabled ? 'Enabled' : 'Not enabled' }}</p><p class="text-xs text-(--color-content-muted)">{{ twoFactorEnabled ? 'Your account requires a second verification step.' : 'Optional — your current sign-in remains unchanged.' }}</p></div></div><UiButton v-if="!twoFactorEnabled" size="sm" :disabled="twoFactorLoading" @click="openTwoFactorSetup">Set up</UiButton><UiButton v-else variant="secondary" size="sm" @click="openDisableTwoFactor">Disable</UiButton></div>
    </UiCard>
    <UiCard title="Appearance" description="Choose how ShortURL looks on this browser."><AppearancePicker /><p class="mt-3 text-xs text-(--color-content-muted)">System automatically follows your device appearance setting.</p></UiCard>
    <UiCard :title="t('preferences')" :description="t('preferencesDescription')"><form class="grid gap-4 sm:grid-cols-2" @submit.prevent="savePreferences"><UiSelect v-model="preferenceLanguage" :label="t('language')" :options="languageOptions" :disabled="preferencesSaving" :error="preferenceErrors.language" /><UiSelect v-model="preferenceTimezone" :label="t('timezone')" :options="timezoneSelectOptions" searchable search-placeholder="Search timezone…" :disabled="preferencesSaving" :error="preferenceErrors.timezone" /><div class="sm:col-span-2"><UiButton type="submit" :loading="preferencesSaving" :disabled="!preferencesDirty">{{ t('savePreferences') }}</UiButton></div></form></UiCard>
    <UiModal v-model="passwordOpen" title="Change your password" description="Use at least 10 characters. Every other signed-in session will be ended."><form class="flex flex-col gap-4" @submit.prevent="changePassword"><UiInput v-model="currentPassword" label="Current password" type="password" autocomplete="current-password" required :disabled="passwordSaving" :error="passwordErrors.current" /><UiInput v-model="newPassword" label="New password" type="password" autocomplete="new-password" required hint="At least 10 characters" :disabled="passwordSaving" :error="passwordErrors.next" /><UiInput v-model="confirmPassword" label="Confirm new password" type="password" autocomplete="new-password" required :disabled="passwordSaving" :error="passwordErrors.confirm" /></form><template #actions><UiButton variant="secondary" :disabled="passwordSaving" @click="passwordOpen = false">Cancel</UiButton><UiButton :loading="passwordSaving" @click="changePassword">Update password</UiButton></template></UiModal>
    <UiModal v-model="twoFactorOpen" title="Set up two-step verification" :description="twoFactorSetup ? 'Scan the QR code, save your recovery codes, then verify one code.' : 'Confirm your password to begin.'" size="lg">
      <form v-if="!twoFactorSetup" class="flex flex-col gap-4" @submit.prevent="beginTwoFactor"><UiInput v-model="twoFactorPassword" label="Current password" type="password" autocomplete="current-password" required :error="twoFactorError" /></form>
      <div v-else class="grid gap-5 md:grid-cols-[14rem_1fr]"><div class="flex flex-col items-center gap-2"><img :src="twoFactorQr" alt="Authenticator QR code" class="size-52 rounded-xl bg-white p-2"><p class="text-center text-xs text-(--color-content-muted)">Scan with your authenticator app</p></div><div class="space-y-4"><div><p class="text-sm font-semibold">Manual setup key</p><UiCopyButton class="mt-1" :value="twoFactorSetup.secret" show-value label="Copy" /></div><div><div class="flex items-center justify-between"><p class="text-sm font-semibold">Recovery codes</p><UiCopyButton :value="twoFactorSetup.recovery_codes.join('\n')" label="Copy all" /></div><p class="mt-1 text-xs text-(--color-content-muted)">Store these somewhere safe. Each code works once.</p><div class="mt-2 grid grid-cols-2 gap-1.5 rounded-lg bg-(--color-surface-muted) p-3"><code v-for="recovery in twoFactorSetup.recovery_codes" :key="recovery" class="text-xs">{{ recovery }}</code></div></div><UiInput v-model="twoFactorCode" label="Authentication code" inputmode="numeric" autocomplete="one-time-code" placeholder="123456" required :error="twoFactorError" /></div></div>
      <template #actions><UiButton variant="secondary" :disabled="twoFactorBusy" @click="twoFactorOpen = false">Cancel</UiButton><UiButton :loading="twoFactorBusy" @click="twoFactorSetup ? enableTwoFactor() : beginTwoFactor()">{{ twoFactorSetup ? 'Enable verification' : 'Continue' }}</UiButton></template>
    </UiModal>
    <UiModal v-model="disableTwoFactorOpen" title="Disable two-step verification?" description="Your account will return to password-only sign in." danger><form class="flex flex-col gap-4" @submit.prevent="disableTwoFactor"><UiInput v-model="disablePassword" label="Current password" type="password" autocomplete="current-password" required /><UiInput v-model="disableCode" label="Authentication or recovery code" autocomplete="one-time-code" required :error="disableError" /></form><template #actions><UiButton variant="secondary" :disabled="twoFactorBusy" @click="disableTwoFactorOpen = false">Cancel</UiButton><UiButton variant="danger" :loading="twoFactorBusy" @click="disableTwoFactor">Disable verification</UiButton></template></UiModal>
  </div>
</template>
