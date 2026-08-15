<script setup lang="ts">
import type { SystemBranding } from '~/types/api'
import { DEFAULT_BRANDING } from '~/composables/useBranding'
import { ApiError } from '~/composables/useApi'

definePageMeta({ middleware: 'auth' })
useHead({ title: 'Whitelabeling · ShortURL' })

const session = useSession()
const { branding, load, set } = useBranding()
const { branding: brandingService } = useServices()
const config = useRuntimeConfig()
const toast = useToast()
const loading = ref(true)
const saving = ref(false)
const uploading = ref<string | null>(null)
const resetOpen = ref(false)
const form = reactive<SystemBranding>({ ...DEFAULT_BRANDING })
const isAdmin = computed(() => Boolean(session.user.value?.is_admin))

const assets = computed(() => [
  { kind: 'logo_light' as const, label: 'Logo for light surfaces', hint: 'Wide logo, shown in light mode.', url: form.logo_light_url, dark: false },
  { kind: 'logo_dark' as const, label: 'Logo for dark surfaces', hint: 'Wide logo, shown in dark mode.', url: form.logo_dark_url, dark: true },
  { kind: 'logo_compact' as const, label: 'Compact logo', hint: 'Square mark for mobile and narrow areas.', url: form.logo_compact_url, dark: false },
  { kind: 'favicon' as const, label: 'Favicon', hint: 'Square browser icon; PNG or ICO recommended.', url: form.favicon_url, dark: false },
])

onMounted(async () => {
  if (isAdmin.value) Object.assign(form, await load(true))
  loading.value = false
})

async function saveBranding() {
  saving.value = true
  try {
    const updated = await brandingService.update({ ...form })
    Object.assign(form, updated)
    set(updated)
    toast.success('Whitelabeling settings saved')
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not save whitelabeling settings.')
  } finally { saving.value = false }
}

async function uploadAsset(kind: typeof assets.value[number]['kind'], event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  if (!file) return
  if (file.size > 2 * 1024 * 1024) { toast.error('Asset must be 2 MB or smaller.'); input.value = ''; return }
  uploading.value = kind
  try {
    const body = new FormData(); body.append('file', file)
    const response = await fetch(`${config.public.apiBaseUrl}/system/branding/assets/${kind}`, { method: 'POST', credentials: 'same-origin', body })
    const payload = await response.json()
    if (!response.ok) throw new Error(payload?.error?.message || 'Upload failed')
    Object.assign(form, payload.data)
    set(payload.data)
    toast.success('Brand asset uploaded')
  } catch (error) { toast.error(error instanceof Error ? error.message : 'Could not upload asset.') }
  finally { uploading.value = null; input.value = '' }
}

async function removeAsset(kind: typeof assets.value[number]['kind']) {
  uploading.value = kind
  try {
    await brandingService.removeAsset(kind)
    const updated = await load(true)
    Object.assign(form, updated); set(updated)
    toast.success('Brand asset removed')
  } catch (error) { toast.error(error instanceof ApiError ? error.message : 'Could not remove asset.') }
  finally { uploading.value = null }
}

async function resetBranding() {
  saving.value = true
  try {
    await Promise.all(assets.value.map(asset => brandingService.removeAsset(asset.kind).catch(() => undefined)))
    const updated = await brandingService.update({ ...DEFAULT_BRANDING })
    Object.assign(form, updated); set(updated); resetOpen.value = false
    toast.success('Default ShortURL whitelabeling restored')
  } catch (error) { toast.error(error instanceof ApiError ? error.message : 'Could not reset whitelabeling.') }
  finally { saving.value = false }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-wrap items-start justify-between gap-3">
      <div><p class="mb-1 text-sm font-semibold text-(--color-accent)">System</p><h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Whitelabeling</h1><p class="text-sm text-(--color-content-muted)">Customize this entire installation for your organization.</p></div>
      <UiButton v-if="isAdmin" variant="secondary" @click="resetOpen = true">Reset defaults</UiButton>
    </header>

    <UiCard v-if="!isAdmin" :padded="false"><UiEmptyState title="System administrator access required" description="Only installation administrators can change whitelabeling settings." /></UiCard>
    <div v-else-if="loading" class="space-y-4"><UiSkeleton v-for="item in 4" :key="item" height="12rem" rounded="lg" /></div>
    <form v-else class="flex flex-col gap-5" @submit.prevent="saveBranding">
      <WhitelabelPreview :value="form" />
      <UiCard title="Brand identity" description="Names shown throughout the dashboard and public authentication pages.">
        <div class="grid gap-4 sm:grid-cols-2"><UiInput v-model="form.app_name" label="Application name" required maxlength="80" /><UiInput v-model="form.organization_name" label="Organization name" placeholder="Acme Corporation" /></div>
        <div class="mt-4"><UiInput v-model="form.tagline" label="Tagline" /></div>
        <div class="mt-4"><UiCheckbox v-model="form.show_powered_by">Show “Powered by ShortURL” when using a custom name</UiCheckbox></div>
      </UiCard>

      <UiCard title="Brand assets" description="PNG, JPEG, WebP, or ICO. Maximum 2 MB per file.">
        <div class="grid gap-3 sm:grid-cols-2">
          <div v-for="asset in assets" :key="asset.kind" class="rounded-lg border border-(--color-border) p-3">
            <div class="grid h-20 place-items-center rounded-md p-3" :class="asset.dark ? 'bg-slate-950' : 'bg-white'">
              <img v-if="asset.url" :src="asset.url" :alt="asset.label" class="max-h-14 max-w-full object-contain"><Icon v-else name="lucide:image" size="24" class="text-slate-400" />
            </div>
            <p class="mt-3 text-sm font-semibold">{{ asset.label }}</p><p class="text-xs text-(--color-content-muted)">{{ asset.hint }}</p>
            <div class="mt-3 flex gap-2"><label class="inline-flex cursor-pointer items-center rounded-md border border-(--color-border-strong) px-2.5 py-1.5 text-xs font-semibold hover:bg-(--color-surface-muted)"><input type="file" class="sr-only" accept="image/png,image/jpeg,image/webp,image/x-icon,.ico" :disabled="uploading === asset.kind" @change="uploadAsset(asset.kind, $event)">{{ uploading === asset.kind ? 'Uploading…' : asset.url ? 'Replace' : 'Upload' }}</label><UiButton v-if="asset.url" variant="ghost" size="sm" :disabled="Boolean(uploading)" @click="removeAsset(asset.kind)"><span class="text-(--color-danger)">Remove</span></UiButton></div>
          </div>
        </div>
      </UiCard>

      <UiCard title="Brand experience" description="Global colors and authentication-page messaging.">
        <div class="grid gap-4 sm:grid-cols-2"><label class="text-sm font-medium">Primary color<div class="mt-1.5 flex items-center gap-2"><input v-model="form.primary_color" type="color" class="size-10 rounded"><UiInput v-model="form.primary_color" /></div></label><label class="text-sm font-medium">Navigation color<div class="mt-1.5 flex items-center gap-2"><input v-model="form.shell_color" type="color" class="size-10 rounded"><UiInput v-model="form.shell_color" /></div></label></div>
        <div class="mt-4 grid gap-4"><UiInput v-model="form.login_heading" label="Login heading" /><div><label class="mb-1.5 block text-sm font-medium">Login description</label><textarea v-model="form.login_description" rows="3" class="w-full rounded-md border border-(--color-border) bg-(--color-surface-raised) px-3 py-2 text-sm outline-none focus:border-(--color-accent)" /></div><UiInput v-model="form.footer_text" label="Footer text" /></div>
      </UiCard>

      <UiCard title="Support and legal links" description="Optional destinations shown where users need help or policy information."><div class="grid gap-4 sm:grid-cols-2"><UiInput v-model="form.support_email" label="Support email" type="email" /><UiInput v-model="form.support_url" label="Support URL" type="url" /><UiInput v-model="form.documentation_url" label="Documentation URL" type="url" /><UiInput v-model="form.privacy_url" label="Privacy policy URL" type="url" /><UiInput v-model="form.terms_url" label="Terms URL" type="url" /></div></UiCard>

      <div class="sticky bottom-20 z-10 flex justify-end rounded-xl border border-(--color-border) bg-(--color-surface-raised)/95 p-3 shadow-lg backdrop-blur lg:bottom-4"><UiButton type="submit" :loading="saving">Save whitelabeling</UiButton></div>
    </form>

    <UiModal v-model="resetOpen" title="Reset all whitelabeling?" description="Names, colors, copy, links, logos, and favicon will return to the ShortURL defaults." danger><template #actions><UiButton variant="secondary" :disabled="saving" @click="resetOpen = false">Cancel</UiButton><UiButton variant="danger" :loading="saving" @click="resetBranding">Reset whitelabeling</UiButton></template></UiModal>
  </div>
</template>
