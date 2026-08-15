<script setup lang="ts">
import type { Domain, PagePreview } from '~/types/api'
import type { LinkFormErrors } from './form'
import { emptyLinkForm, optional, toRfc3339 } from './form'
import { ApiError } from '~/composables/useApi'

const ws = useWorkspaces()
const toast = useToast()
const { links, domains } = useServices()
const modal = useCreateLinkModal()
const activeDomains = ref<Domain[]>([])
const loading = ref(false)
const loadError = ref<string | null>(null)
const form = ref(emptyLinkForm())
const errors = ref<LinkFormErrors>({})
const submitting = ref(false)
const preview = ref<PagePreview | null>(null)
const previewing = ref(false)
const previewError = ref<string | null>(null)
let previewTimer: ReturnType<typeof setTimeout> | undefined
let previewToken = 0
let autofilledTitle = ''

async function loadDomains() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) return
  loading.value = true
  loadError.value = null
  try {
    const result = await domains.list(workspaceId)
    activeDomains.value = result.data.filter(domain => domain.status === 'active')
    const preferred = activeDomains.value.find(domain => domain.is_default) ?? activeDomains.value[0]
    form.value.domain_id = preferred?.id ?? ''
  } catch (error) {
    loadError.value = error instanceof ApiError ? error.message : 'Could not load domains.'
  } finally {
    loading.value = false
  }
}

watch(modal.open, (open) => {
  if (!open) {
    clearTimeout(previewTimer)
    return
  }
  form.value = emptyLinkForm()
  errors.value = {}
  preview.value = null
  previewError.value = null
  autofilledTitle = ''
  loadDomains()
})

watch(() => form.value.destination_url, (raw) => {
  clearTimeout(previewTimer)
  const token = ++previewToken
  preview.value = null
  previewError.value = null
  previewing.value = false
  const destination = raw.trim()
  if (!modal.open.value || !/^https?:\/\//i.test(destination)) return
  previewTimer = setTimeout(async () => {
    const workspaceId = ws.activeId.value
    if (!workspaceId) return
    previewing.value = true
    try {
      const result = await links.preview(workspaceId, destination)
      if (token !== previewToken) return
      preview.value = result
      if (result.title && (!form.value.title.trim() || form.value.title === autofilledTitle)) {
        form.value.title = result.title
        autofilledTitle = result.title
      }
    } catch {
      if (token === previewToken) previewError.value = 'Preview unavailable; the link can still be created.'
    } finally {
      if (token === previewToken) previewing.value = false
    }
  }, 650)
})

onBeforeUnmount(() => clearTimeout(previewTimer))

const conflicts: Record<string, { field?: keyof LinkFormErrors, message: string }> = {
  slug_taken: { field: 'slug', message: 'That slug is already in use on this domain.' },
  no_default_domain: { field: 'domain_id', message: 'Choose a domain or set a default domain first.' },
  domain_not_active: { field: 'domain_id', message: 'That domain is not verified yet.' },
  external_reference_taken: { message: 'That external reference is already in use.' },
}

function applyError(error: unknown) {
  errors.value = {}
  if (!(error instanceof ApiError)) {
    errors.value.form = 'Something went wrong. Please try again.'
    return
  }
  const fields = ['destination_url', 'domain_id', 'slug', 'title', 'redirect_type', 'expires_at', 'password', 'max_clicks'] as const
  let placed = false
  for (const field of fields) {
    const message = error.field(field)
    if (message) { errors.value[field] = message; placed = true }
  }
  const conflict = conflicts[error.code]
  if (conflict) {
    if (conflict.field) errors.value[conflict.field] = conflict.message
    else errors.value.form = conflict.message
  } else if (!placed) errors.value.form = error.message
}

async function submit() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || submitting.value) return
  const model = form.value
  const destination = model.destination_url.trim()
  if (!destination) { errors.value = { destination_url: 'A destination URL is required.' }; return }
  const body: Parameters<typeof links.create>[1] = { destination_url: destination }
  if (model.domain_id) body.domain_id = model.domain_id
  const slug = optional(model.slug); if (slug) body.slug = slug
  const title = optional(model.title); if (title) body.title = title
  if (preview.value) body.metadata = { preview: preview.value }
  const redirectType = Number(model.redirect_type); if (redirectType > 0) body.redirect_type = redirectType
  const expiresAt = toRfc3339(model.expires_at); if (expiresAt) body.expires_at = expiresAt
  const password = optional(model.password); if (password) body.password = password
  const rawMaxClicks = String(model.max_clicks ?? '').trim()
  const maxClicks = Number(rawMaxClicks)
  if (rawMaxClicks && Number.isInteger(maxClicks) && maxClicks > 0) body.max_clicks = maxClicks
  submitting.value = true
  try {
    const link = await links.create(workspaceId, body)
    modal.hide()
    toast.success(`Created ${link.short_url}`)
    await navigateTo(`/dashboard/links/${link.id}`)
  } catch (error) {
    applyError(error)
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <UiModal v-model="modal.open.value" title="Create a short link" description="Turn a long destination into a link you can share and track." size="lg">
    <div v-if="loading" class="space-y-4 py-2" role="status" aria-label="Loading link form">
      <div v-for="field in 4" :key="field" class="space-y-2">
        <UiSkeleton height="0.7rem" width="7rem" />
        <UiSkeleton height="2.25rem" rounded="lg" />
      </div>
    </div>
    <div v-else-if="loadError" class="py-6 text-center">
      <p class="text-sm text-(--color-danger)">{{ loadError }}</p>
      <UiButton class="mt-3" variant="secondary" size="sm" @click="loadDomains">Try again</UiButton>
    </div>
    <UiEmptyState v-else-if="!activeDomains.length" title="Add a domain first" description="Connect and verify a domain before creating short links.">
      <UiButton @click="modal.hide(); navigateTo('/dashboard/domains')">Go to Domains</UiButton>
    </UiEmptyState>
    <form v-else class="flex flex-col gap-4" @submit.prevent="submit">
      <DashboardFormAlert v-if="errors.form">{{ errors.form }}</DashboardFormAlert>
      <LinksFormFields v-model="form" :domains="activeDomains" :errors="errors" :disabled="submitting" />
      <div v-if="previewing" class="flex gap-3 rounded-xl border border-(--color-border) p-3" role="status" aria-label="Loading page preview">
        <UiSkeleton height="3rem" width="3rem" rounded="lg" />
        <div class="flex-1 space-y-2"><UiSkeleton width="45%" /><UiSkeleton height="0.75rem" width="80%" /></div>
      </div>
      <LinksPreviewCard v-else-if="preview" :preview="preview" :destination-url="form.destination_url.trim()" />
      <p v-else-if="previewError" class="text-xs text-(--color-content-muted)">{{ previewError }}</p>
    </form>
    <template #actions>
      <UiButton variant="secondary" :disabled="submitting" @click="modal.hide()">Cancel</UiButton>
      <UiButton :loading="submitting" :disabled="loading || !!loadError || !activeDomains.length" @click="submit">Create link</UiButton>
    </template>
  </UiModal>
</template>
