<script setup lang="ts">
import type { Domain, Link } from '~/types/api'
import type { LinkFormErrors } from '~/components/links/form'
import type { CustomRange, LinkAnalytics, PresetRange } from '~/components/analytics/types'
import AnalyticsChart from '~/components/analytics/Chart.vue'
import AnalyticsRangePicker from '~/components/analytics/RangePicker.vue'
import LinkFormFields from '~/components/links/FormFields.vue'
import LinkStatusBadge from '~/components/links/StatusBadge.vue'
import { emptyLinkForm, linkToForm, toRfc3339 } from '~/components/links/form'
import { formatDateTime, formatNumber, truncateMiddle } from '~/components/links/format'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

const route = useRoute()

const ws = useWorkspaces()
const toast = useToast()
const { links, domains, analytics } = useServices()

const linkId = computed(() => String(route.params.id))

// -------------------------------------------------------------- the link

const link = ref<Link | null>(null)
const qrOpen = ref(false)

useHead({
  title: () => (link.value ? `${link.value.domain}/${link.value.slug} · ShortURL` : 'Link · ShortURL'),
})

const allDomains = ref<Domain[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)

const form = ref(emptyLinkForm())
/** The form as it was when loaded, so the patch can send only what changed. */
const baseline = ref(emptyLinkForm())
const errors = ref<LinkFormErrors>({})
const saving = ref(false)

const formDomains = computed(() => {
  const list = allDomains.value.filter(d => d.status === 'active')
  const current = allDomains.value.find(d => d.id === link.value?.domain_id)
  // A link can outlive its domain's active status; keep it selectable so the
  // select does not silently reassign the link to another domain on save.
  if (current && !list.some(d => d.id === current.id)) list.unshift(current)
  return list
})

const advancedOpen = computed(() => {
  const current = link.value
  if (!current) return false
  return Boolean(current.expires_at || current.max_clicks || current.has_password
    || (current.redirect_type && current.redirect_type !== 302))
})

function resetForm(current: Link) {
  form.value = linkToForm(current)
  baseline.value = linkToForm(current)
  errors.value = {}
}

async function load() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    loading.value = false
    loadError.value = 'No workspace selected.'
    return
  }

  loading.value = true
  loadError.value = null
  try {
    const [current, domainList] = await Promise.all([
      links.get(workspaceId, linkId.value),
      domains.list(workspaceId).then(r => r.data).catch(() => [] as Domain[]),
    ])
    link.value = current
    allDomains.value = domainList
    resetForm(current)
  } catch (error) {
    loadError.value = error instanceof ApiError
      ? (error.status === 404 ? 'That link does not exist in this workspace.' : error.message)
      : 'Could not load the link.'
  } finally {
    loading.value = false
  }
}

watch([linkId, () => ws.activeId.value], () => load(), { immediate: true })

// ------------------------------------------------------------------ save

const CONFLICTS: Record<string, { field?: keyof LinkFormErrors, message: string }> = {
  slug_taken: {
    field: 'slug',
    message: 'That slug is already in use on this domain. Pick another one.',
  },
  no_default_domain: {
    field: 'domain_id',
    message: 'This workspace has no default domain. Choose a domain above, or set one as the default on the Domains page.',
  },
  domain_not_active: {
    field: 'domain_id',
    message: 'That domain is not verified yet. Verify it on the Domains page first.',
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

/**
 * Only changed fields, and nullable fields cleared with an explicit `null`.
 *
 * The server distinguishes an absent key from `null`: absent means "leave it",
 * `null` means "clear it". Sending `undefined` for an emptied expiry would
 * quietly keep the old one, which is the bug this function exists to avoid.
 */
function buildPatch(): Record<string, unknown> | null {
  const next = form.value
  const prev = baseline.value
  const patch: Record<string, unknown> = {}

  const destination = next.destination_url.trim()
  if (destination !== prev.destination_url) patch.destination_url = destination

  const slug = next.slug.trim()
  if (slug !== prev.slug) patch.slug = slug

  if (next.domain_id !== prev.domain_id) patch.domain_id = next.domain_id

  if (next.redirect_type !== prev.redirect_type) patch.redirect_type = Number(next.redirect_type)

  const title = next.title.trim()
  if (title !== prev.title) patch.title = title === '' ? null : title

  if (next.expires_at !== prev.expires_at) {
    if (next.expires_at === '') {
      patch.expires_at = null
    } else {
      const iso = toRfc3339(next.expires_at)
      if (!iso) {
        errors.value = { expires_at: 'That is not a valid date and time.' }
        return null
      }
      patch.expires_at = iso
    }
  }

  const maxClicks = String(next.max_clicks ?? '').trim()
  const previousMaxClicks = String(prev.max_clicks ?? '').trim()
  if (maxClicks !== previousMaxClicks) {
    if (maxClicks === '') {
      patch.max_clicks = null
    } else {
      const parsed = Number(maxClicks)
      if (!Number.isInteger(parsed) || parsed <= 0) {
        errors.value = { max_clicks: 'Enter a positive number, or leave it empty for no limit.' }
        return null
      }
      patch.max_clicks = parsed
    }
  }

  if (next.remove_password) patch.password = null
  else if (next.password !== '') patch.password = next.password

  return patch
}

async function save() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || saving.value) return

  errors.value = {}
  const patch = buildPatch()
  if (!patch) return

  if (Object.keys(patch).length === 0) {
    toast.info('Nothing to save — no fields changed.')
    return
  }

  saving.value = true
  try {
    const updated = await links.update(workspaceId, linkId.value, patch)
    link.value = updated
    resetForm(updated)
    toast.success('Link saved')
  } catch (error) {
    applyError(error)
  } finally {
    saving.value = false
  }
}

// ---------------------------------------------------------- enable/disable

const togglingStatus = ref(false)

async function toggleStatus() {
  const workspaceId = ws.activeId.value
  const current = link.value
  if (!workspaceId || !current || togglingStatus.value) return

  const next = current.status === 'active' ? 'disabled' : 'active'
  togglingStatus.value = true
  try {
    const updated = await links.update(workspaceId, current.id, { status: next })
    link.value = updated
    baseline.value = linkToForm(updated)
    toast.success(next === 'active' ? 'Link enabled' : 'Link disabled')
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not change the status.')
  } finally {
    togglingStatus.value = false
  }
}

// ------------------------------------------------------------- analytics

const range = ref<PresetRange>('7d')
const customRange = ref<CustomRange | null>(null)
const stats = ref<LinkAnalytics | null>(null)
const statsLoading = ref(true)
const statsError = ref<string | null>(null)

async function loadStats() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) return

  statsLoading.value = true
  statsError.value = null
  try {
    // The service types this as ClicksReport, but the server returns the
    // per-link DTO: link, totals and a series, with no dimension breakdowns.
    const res = await analytics.forLink(workspaceId, linkId.value, {
      range: range.value,
      ...(range.value === 'custom' && customRange.value ? customRange.value : {}),
    })
    stats.value = res as unknown as LinkAnalytics
  } catch (error) {
    statsError.value = error instanceof ApiError ? error.message : 'Could not load analytics.'
    stats.value = null
  } finally {
    statsLoading.value = false
  }
}

watch([linkId, range, () => ws.activeId.value], () => loadStats(), { immediate: true })

function applyCustomRange(value: CustomRange) {
  customRange.value = value
  if (range.value === 'custom') loadStats()
}

const series = computed(() => stats.value?.series ?? [])
const hasSeriesData = computed(() => series.value.some(p => p.clicks > 0))

// --------------------------------------------------------------- delete

const deleteOpen = ref(false)
const deleting = ref(false)

async function confirmDelete() {
  const workspaceId = ws.activeId.value
  const current = link.value
  if (!workspaceId || !current) return

  deleting.value = true
  try {
    await links.remove(workspaceId, current.id)
    toast.success(`Deleted ${current.short_url}`)
    await navigateTo('/dashboard/links')
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not delete the link.')
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <NuxtLink to="/dashboard/links" class="text-sm text-(--color-content-muted) hover:underline">
      &larr; Links
    </NuxtLink>

    <div v-if="loading" class="flex flex-col gap-5" role="status" aria-label="Loading link details">
      <div class="space-y-3"><UiSkeleton height="2rem" width="45%" /><UiSkeleton height="0.8rem" width="70%" /><UiSkeleton height="0.65rem" width="55%" /></div>
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-5"><UiSkeleton width="6rem" /><UiSkeleton class="mt-5" height="16rem" rounded="lg" /></div>
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-5"><UiSkeleton width="5rem" /><div class="mt-5 space-y-4"><UiSkeleton v-for="field in 4" :key="field" height="2.25rem" rounded="lg" /></div></div>
    </div>

    <UiCard v-else-if="loadError || !link">
      <p class="text-sm text-(--color-danger)" role="alert">
        {{ loadError ?? 'Link not found.' }}
      </p>
      <UiButton class="mt-3" variant="secondary" size="sm" to="/dashboard/links">
        Back to links
      </UiButton>
    </UiCard>

    <template v-else>
      <!-- Header -->
      <header class="flex flex-wrap items-start justify-between gap-4">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <a
              :href="link.short_url"
              target="_blank"
              rel="noopener noreferrer"
              class="break-all text-2xl font-semibold tracking-tight hover:underline"
            >{{ link.domain }}/{{ link.slug }}</a>
            <UiCopyButton :value="link.short_url" label="Copy" />
            <UiButton variant="ghost" size="sm" @click="qrOpen = true">
              <Icon name="lucide:qr-code" size="16" /> QR code
            </UiButton>
            <LinkStatusBadge :status="link.status" />
          </div>
          <p class="mt-1 break-all text-sm text-(--color-content-muted)" :title="link.destination_url">
            &rarr; {{ truncateMiddle(link.destination_url, 96) }}
          </p>
          <p class="mt-1 text-xs text-(--color-content-subtle)">
            Created {{ formatDateTime(link.created_at) }}
            <span v-if="link.created_via === 'api'"> · via API</span>
            <span v-if="link.expires_at"> · expires {{ formatDateTime(link.expires_at) }}</span>
            <span v-if="link.max_clicks"> · limit {{ formatNumber(link.max_clicks) }} clicks</span>
            <span v-if="link.has_password"> · password protected</span>
          </p>
        </div>

        <UiButton
          variant="secondary"
          :loading="togglingStatus"
          @click="toggleStatus"
        >
          {{ link.status === 'active' ? 'Disable link' : 'Enable link' }}
        </UiButton>
      </header>

      <!-- Analytics -->
      <UiCard title="Clicks">
        <template #actions>
          <AnalyticsRangePicker
            v-model="range"
            :custom-range="customRange"
            :disabled="statsLoading"
            @custom="applyCustomRange"
          />
        </template>

        <div v-if="statsLoading" class="space-y-5 py-2" role="status" aria-label="Loading link analytics">
          <div class="flex gap-8"><UiSkeleton height="2.5rem" width="8rem" /><UiSkeleton height="2.5rem" width="6rem" /></div>
          <UiSkeleton height="16rem" rounded="lg" />
        </div>
        <p v-else-if="statsError" class="py-6 text-center text-sm text-(--color-danger)" role="alert">
          {{ statsError }}
        </p>
        <template v-else-if="stats">
          <dl class="mb-5 flex flex-wrap gap-8">
            <div>
              <dt class="text-xs uppercase tracking-wide text-(--color-content-subtle)">
                All-time clicks
              </dt>
              <dd class="text-2xl font-semibold tabular-nums">
                {{ formatNumber(stats.total_clicks) }}
              </dd>
            </div>
            <div>
              <dt class="text-xs uppercase tracking-wide text-(--color-content-subtle)">
                Today
              </dt>
              <dd class="text-2xl font-semibold tabular-nums">
                {{ formatNumber(stats.clicks_today) }}
              </dd>
            </div>
          </dl>

          <!-- No chart shell when there is nothing to plot. -->
          <AnalyticsChart
            v-if="hasSeriesData"
            :points="series"
            :granularity="stats.granularity"
            label="Clicks for this link"
          />
          <p v-else class="rounded-md bg-(--color-surface-muted) px-4 py-6 text-center text-sm text-(--color-content-muted)">
            No clicks in this range yet. Share the short URL and the chart fills in.
          </p>
        </template>
      </UiCard>

      <!-- Edit -->
      <UiCard title="Edit link" description="Only the fields you change are sent.">
        <form class="flex flex-col gap-5" @submit.prevent="save">
          <div
            v-if="errors.form"
            class="rounded-md border border-(--color-danger) bg-red-500/10 px-3 py-2 text-sm text-(--color-danger)"
            role="alert"
          >
            {{ errors.form }}
          </div>

          <LinkFormFields
            v-model="form"
            :domains="formDomains"
            :errors="errors"
            :has-password="link.has_password"
            :advanced-open="advancedOpen"
            :disabled="saving"
          />

          <div class="flex items-center gap-2">
            <UiButton type="submit" :loading="saving">
              Save changes
            </UiButton>
            <UiButton variant="ghost" :disabled="saving" @click="resetForm(link)">
              Reset
            </UiButton>
          </div>
        </form>
      </UiCard>

      <!-- Delete -->
      <UiCard title="Danger zone">
        <div class="rounded-xl border border-red-500/30 bg-red-500/5 p-4">
          <h3 class="text-sm font-medium text-(--color-danger)">
            Delete link
          </h3>
          <p class="mt-1 text-sm text-(--color-content-muted)">
            Deleting this link stops the short URL resolving immediately and removes its click history.
          </p>
          <div class="mt-3">
            <UiButton variant="danger" size="sm" @click="deleteOpen = true">
              Delete link
            </UiButton>
          </div>
        </div>
      </UiCard>

      <UiModal
        v-model="deleteOpen"
        title="Delete this link?"
        description="The short URL stops resolving immediately and its click history is removed. This cannot be undone."
        danger
      >
        <p class="rounded-md bg-(--color-surface-muted) px-3 py-2 font-mono text-xs">
          {{ link.short_url }}
        </p>

        <template #actions>
          <UiButton variant="secondary" @click="deleteOpen = false">
            Cancel
          </UiButton>
          <UiButton variant="danger" :loading="deleting" @click="confirmDelete">
            Delete link
          </UiButton>
        </template>
      </UiModal>

      <LinksQrModal
        v-model="qrOpen"
        :url="link.short_url"
        :label="link.title || `${link.domain}/${link.slug}`"
      />
    </template>
  </div>
</template>
