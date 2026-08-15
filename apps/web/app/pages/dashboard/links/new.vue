<script setup lang="ts">
import type { Domain, PagePreview } from '~/types/api'
import type { LinkFormErrors } from '~/components/links/form'
import LinkFormFields from '~/components/links/FormFields.vue'
import { emptyLinkForm, optional, toRfc3339 } from '~/components/links/form'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'New link · ShortURL' })

const ws = useWorkspaces()
const toast = useToast()
const { links, domains } = useServices()

const activeDomains = ref<Domain[]>([])
const loadingDomains = ref(true)
const domainsError = ref<string | null>(null)

const form = ref(emptyLinkForm())
const errors = ref<LinkFormErrors>({})
const submitting = ref(false)
const preview = ref<PagePreview | null>(null)
const previewing = ref(false)
const previewError = ref<string | null>(null)
let previewTimer: ReturnType<typeof setTimeout> | undefined
let previewToken = 0
let autofilledTitle = ''

watch(() => form.value.destination_url, (raw) => {
  clearTimeout(previewTimer)
  const token = ++previewToken
  preview.value = null
  previewError.value = null
  previewing.value = false
  const destination = raw.trim()
  if (!/^https?:\/\//i.test(destination)) return

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

async function loadDomains() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    loadingDomains.value = false
    return
  }
  loadingDomains.value = true
  domainsError.value = null
  try {
    const res = await domains.list(workspaceId)
    activeDomains.value = res.data.filter(d => d.status === 'active')
    // Plan §38: creation defaults to the workspace default domain.
    const preferred = activeDomains.value.find(d => d.is_default) ?? activeDomains.value[0]
    form.value.domain_id = preferred?.id ?? ''
  } catch (error) {
    domainsError.value = error instanceof ApiError ? error.message : 'Could not load domains.'
  } finally {
    loadingDomains.value = false
  }
}

watch(() => ws.activeId.value, () => loadDomains(), { immediate: true })

/**
 * Conflicts arrive as a 409 with a code rather than a field error, so they
 * need placing under the input the user can actually act on.
 */
const CONFLICTS: Record<string, { field?: keyof LinkFormErrors, message: string }> = {
  slug_taken: {
    field: 'slug',
    message: 'That slug is already in use on this domain. Pick another, or leave it empty for a random one.',
  },
  no_default_domain: {
    field: 'domain_id',
    message: 'This workspace has no default domain. Choose a domain above, or set one as the default on the Domains page.',
  },
  domain_not_active: {
    field: 'domain_id',
    message: 'That domain is not verified yet. Verify it on the Domains page before creating links on it.',
  },
  external_reference_taken: {
    message: 'A link with that external reference already exists in this workspace.',
  },
}

const FIELD_KEYS = [
  'destination_url',
  'domain_id',
  'slug',
  'title',
  'redirect_type',
  'expires_at',
  'password',
  'max_clicks',
] as const

function applyError(error: unknown) {
  errors.value = {}

  if (!(error instanceof ApiError)) {
    errors.value.form = 'Something went wrong. Please try again.'
    return
  }

  let placed = false
  for (const key of FIELD_KEYS) {
    const message = error.field(key)
    if (message) {
      errors.value[key] = message
      placed = true
    }
  }

  const conflict = CONFLICTS[error.code]
  if (conflict) {
    if (conflict.field) errors.value[conflict.field] = conflict.message
    else errors.value.form = conflict.message
    return
  }

  if (!placed) errors.value.form = error.message
}

async function submit() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || submitting.value) return

  const model = form.value
  const destination = model.destination_url.trim()
  if (!destination) {
    errors.value = { destination_url: 'A destination URL is required.' }
    return
  }

  // Optional fields are omitted rather than sent as "": an empty string is a
  // value the server would have to validate and reject.
  const body: Parameters<typeof links.create>[1] = { destination_url: destination }
  if (model.domain_id) body.domain_id = model.domain_id

  const slug = optional(model.slug)
  if (slug) body.slug = slug

  const title = optional(model.title)
  if (title) body.title = title
  if (preview.value) body.metadata = { preview: preview.value }

  const redirectType = Number(model.redirect_type)
  if (Number.isFinite(redirectType) && redirectType > 0) body.redirect_type = redirectType

  const expiresAt = toRfc3339(model.expires_at)
  if (expiresAt) body.expires_at = expiresAt

  const password = optional(model.password)
  if (password) body.password = password

  const maxClicks = Number(model.max_clicks)
  if (model.max_clicks.trim() !== '' && Number.isFinite(maxClicks) && maxClicks > 0) {
    body.max_clicks = maxClicks
  }

  submitting.value = true
  errors.value = {}
  try {
    const link = await links.create(workspaceId, body)
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
  <div class="flex flex-col gap-6">
    <header>
      <NuxtLink to="/dashboard/links" class="text-sm text-(--color-content-muted) hover:underline">
        &larr; Links
      </NuxtLink>
      <h1 class="mt-1 text-xl font-semibold tracking-tight">
        New link
      </h1>
    </header>

    <p v-if="loadingDomains" class="text-sm text-(--color-content-muted)" role="status">
      Loading domains…
    </p>

    <UiCard v-else-if="domainsError">
      <p class="text-sm text-(--color-danger)" role="alert">
        {{ domainsError }}
      </p>
      <UiButton class="mt-3" variant="secondary" size="sm" @click="loadDomains">
        Try again
      </UiButton>
    </UiCard>

    <!-- Without a verified domain there is nothing to build a short URL on, so
         the form would only be able to fail. -->
    <UiCard v-else-if="!activeDomains.length" :padded="false">
      <UiEmptyState
        title="Add a domain first"
        description="Short links live on a verified custom domain. Connect one and verify its DNS records, then come back to create links."
      >
        <UiButton to="/dashboard/domains">
          Go to Domains
        </UiButton>
      </UiEmptyState>
    </UiCard>

    <UiCard v-else>
      <form class="flex flex-col gap-5" @submit.prevent="submit">
        <div
          v-if="errors.form"
          class="rounded-md border border-(--color-danger) bg-red-500/10 px-3 py-2 text-sm text-(--color-danger)"
          role="alert"
        >
          {{ errors.form }}
        </div>

        <LinkFormFields
          v-model="form"
          :domains="activeDomains"
          :errors="errors"
          :disabled="submitting"
        />

        <p v-if="previewing" class="text-sm text-(--color-content-muted)" role="status">
          Reading page metadata…
        </p>
        <LinkPreviewCard
          v-else-if="preview"
          :preview="preview"
          :destination-url="form.destination_url.trim()"
        />
        <p v-else-if="previewError" class="text-xs text-(--color-content-muted)">
          {{ previewError }}
        </p>

        <div class="flex items-center gap-2">
          <UiButton type="submit" :loading="submitting">
            Create link
          </UiButton>
          <UiButton variant="ghost" to="/dashboard/links">
            Cancel
          </UiButton>
        </div>
      </form>
    </UiCard>
  </div>
</template>
