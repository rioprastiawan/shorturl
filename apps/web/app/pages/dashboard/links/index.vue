<script setup lang="ts">
import type { Domain, Link, PagePreview } from '~/types/api'
import LinkRowActions from '~/components/links/RowActions.vue'
import LinkStatusBadge from '~/components/links/StatusBadge.vue'
import { copyText } from '~/components/links/clipboard'
import { formatDate, formatNumber, truncateMiddle } from '~/components/links/format'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Links · ShortURL' })

const ws = useWorkspaces()
const toast = useToast()
const { links, domains } = useServices()
const createLinkModal = useCreateLinkModal()

// ---------------------------------------------------------------- filters

const search = ref('')
const searchInput = ref<HTMLInputElement | null>(null)
const debouncedSearch = ref('')
const status = ref('')
const domainId = ref('')

const STATUS_OPTIONS = [
  { value: '', label: 'All statuses' },
  { value: 'active', label: 'Active' },
  { value: 'disabled', label: 'Disabled' },
  { value: 'archived', label: 'Archived' },
]

// Debounced so typing "newsletter" is one request, not ten.
let searchTimer: ReturnType<typeof setTimeout> | undefined
watch(search, (value) => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => (debouncedSearch.value = value.trim()), 300)
})
onBeforeUnmount(() => {
  clearTimeout(searchTimer)
})

const filtersActive = computed(() =>
  Boolean(debouncedSearch.value || status.value || domainId.value))

function clearFilters() {
  search.value = ''
  debouncedSearch.value = ''
  status.value = ''
  domainId.value = ''
}

function previewOf(link: Link): PagePreview | null {
  const value = link.metadata?.preview
  if (!value || typeof value !== 'object' || !(value as PagePreview).fetched_at) return null
  return value as PagePreview
}

// ------------------------------------------------------------------ data

const items = ref<Link[]>([])
const nextCursor = ref<string | null>(null)
const pending = ref(true)
const loadingMore = ref(false)
const loadError = ref<string | null>(null)
const domainOptions = ref<Domain[]>([])
const domainFilterOptions = computed(() => [
  { value: '', label: 'All domains' },
  ...domainOptions.value.map(domain => ({ value: domain.id, label: domain.hostname })),
])

// A stale response from a filter the user has already changed must not
// overwrite the current one, so every load carries a token.
let requestToken = 0

async function load(more = false) {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    items.value = []
    pending.value = false
    return
  }

  const token = ++requestToken
  if (more) loadingMore.value = true
  else {
    pending.value = true
    loadError.value = null
  }

  try {
    const res = await links.list(workspaceId, {
      search: debouncedSearch.value || undefined,
      status: status.value || undefined,
      domain_id: domainId.value || undefined,
      cursor: more ? (nextCursor.value ?? undefined) : undefined,
    })
    if (token !== requestToken) return
    items.value = more ? [...items.value, ...res.data] : res.data
    // Keyset pagination: the cursor is the only way forward, there is no
    // page count to render.
    nextCursor.value = res.meta?.next_cursor ?? null
  } catch (error) {
    if (token !== requestToken) return
    const message = error instanceof ApiError ? error.message : 'Could not load links.'
    if (more) toast.error(message)
    else loadError.value = message
  } finally {
    if (token === requestToken) {
      pending.value = false
      loadingMore.value = false
    }
  }
}

async function loadDomains() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    domainOptions.value = []
    return
  }
  try {
    const res = await domains.list(workspaceId)
    domainOptions.value = res.data.filter(d => d.status === 'active')
  } catch {
    // The filter is a convenience; losing it must not break the page.
    domainOptions.value = []
  }
}

watch(() => ws.activeId.value, () => {
  domainId.value = ''
  loadDomains()
}, { immediate: true })

watch([() => ws.activeId.value, debouncedSearch, status, domainId], () => load(), {
  immediate: true,
})

// ---------------------------------------------------------------- actions

async function copyShortUrl(link: Link) {
  const ok = await copyText(link.short_url)
  if (ok) toast.success('Copied to clipboard')
  else toast.error('Could not copy to clipboard')
}

const deleteTarget = ref<Link | null>(null)
const deleteOpen = ref(false)
const deleting = ref(false)
const qrTarget = ref<Link | null>(null)
const qrOpen = ref(false)

function showQr(link: Link) {
  qrTarget.value = link
  qrOpen.value = true
}

function askDelete(link: Link) {
  deleteTarget.value = link
  deleteOpen.value = true
}

async function confirmDelete() {
  const target = deleteTarget.value
  const workspaceId = ws.activeId.value
  if (!target || !workspaceId) return

  deleting.value = true
  try {
    await links.remove(workspaceId, target.id)
    items.value = items.value.filter(l => l.id !== target.id)
    toast.success(`Deleted ${target.short_url}`)
    deleteOpen.value = false
    deleteTarget.value = null
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not delete the link.')
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-7">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="mb-1 text-sm font-semibold text-(--color-accent)">Workspace</p>
        <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
          Your links
        </h1>
        <p class="mt-0.5 text-sm text-(--color-content-muted)">
          Create, organize, and track every short link in one place.
        </p>
      </div>
      <UiButton @click="createLinkModal.show()">
        <Icon name="lucide:plus" size="17" /> Create short link
      </UiButton>
    </header>

    <!-- Search and filters are one control surface: search is the primary
         action, while status/domain stay available without competing with it. -->
    <section class="overflow-hidden rounded-2xl border border-(--color-border) bg-(--color-surface-raised) shadow-[0_1px_3px_rgba(16,48,63,0.04)]" aria-label="Link filters">
      <div class="flex items-center gap-3 px-4 py-3.5 sm:px-5">
        <Icon name="lucide:search" size="19" class="shrink-0 text-(--color-content-subtle)" />
        <input
          ref="searchInput"
          data-shortcut-search="links"
          v-model="search"
          type="search"
          aria-label="Search links"
          placeholder="Search title, short link, or destination"
          class="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-(--color-content-subtle) sm:text-base"
        >
        <span
          v-if="!pending && items.length"
          class="hidden shrink-0 rounded-full bg-(--color-surface-muted) px-2.5 py-1 text-xs font-medium tabular-nums text-(--color-content-muted) sm:inline"
        >
          {{ formatNumber(items.length) }} shown
        </span>
        <kbd class="hidden rounded-md border border-(--color-border) bg-(--color-surface-muted) px-2 py-1 font-sans text-[10px] text-(--color-content-subtle) lg:inline">/</kbd>
      </div>

      <div class="flex flex-col gap-3 border-t border-(--color-border) bg-(--color-surface-muted)/45 px-3 py-2.5 sm:flex-row sm:items-center sm:justify-between sm:px-4">
        <div class="flex min-w-0 items-center gap-1 overflow-x-auto" role="group" aria-label="Filter by status">
          <button
            v-for="option in STATUS_OPTIONS"
            :key="option.value"
            type="button"
            class="whitespace-nowrap rounded-lg px-3 py-1.5 text-xs font-semibold transition-all"
            :class="status === option.value
              ? 'bg-(--color-surface-raised) text-(--color-content) shadow-sm ring-1 ring-(--color-border)'
              : 'text-(--color-content-muted) hover:bg-(--color-surface-raised)/70 hover:text-(--color-content)'"
            :aria-pressed="status === option.value"
            @click="status = option.value"
          >
            <span
              v-if="option.value"
              class="mr-1.5 inline-block size-1.5 rounded-full"
              :class="option.value === 'active'
                ? 'bg-(--color-success)'
                : option.value === 'disabled'
                  ? 'bg-(--color-warning)'
                  : 'bg-(--color-content-subtle)'"
            />
            {{ option.label }}
          </button>
        </div>

        <div class="flex items-center gap-2 border-t border-(--color-border) pt-2.5 sm:border-l sm:border-t-0 sm:pl-3 sm:pt-0">
          <Icon name="lucide:globe-2" size="15" class="shrink-0 text-(--color-content-subtle)" />
          <UiSelect
            v-model="domainId"
            input-id="filter-domain"
            :options="domainFilterOptions"
            size="sm"
            class="min-w-40 flex-1 sm:w-52 sm:flex-none"
          />
          <button
            v-if="filtersActive"
            type="button"
            class="inline-flex shrink-0 items-center gap-1 rounded-lg px-2.5 py-1.5 text-xs font-semibold text-(--color-content-muted) transition-colors hover:bg-(--color-surface-raised) hover:text-(--color-danger)"
            @click="clearFilters"
          >
            <Icon name="lucide:rotate-ccw" size="13" />
            Reset
          </button>
        </div>
      </div>
    </section>

    <UiCard :padded="false">
      <p v-if="pending" class="px-5 py-10 text-center text-sm text-(--color-content-muted)" role="status">
        Loading links…
      </p>

      <div v-else-if="loadError" class="px-5 py-10 text-center" role="alert">
        <p class="text-sm text-(--color-danger)">
          {{ loadError }}
        </p>
        <UiButton class="mt-3" variant="secondary" size="sm" @click="load()">
          Try again
        </UiButton>
      </div>

      <UiEmptyState
        v-else-if="!items.length && filtersActive"
        title="No links match these filters"
        description="Nothing in this workspace matches the current search, status and domain."
      >
        <UiButton variant="secondary" @click="clearFilters">
          Clear filters
        </UiButton>
      </UiEmptyState>

      <UiEmptyState
        v-else-if="!items.length"
        title="No links yet"
        description="Short links you create appear here with their click counts."
      >
        <UiButton @click="createLinkModal.show()">
          Create your first link
        </UiButton>
      </UiEmptyState>

      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[56rem] text-left text-sm">
          <thead>
            <tr class="border-b border-(--color-border) text-xs uppercase tracking-wide text-(--color-content-subtle)">
              <th scope="col" class="px-5 py-3 font-medium">
                Short URL
              </th>
              <th scope="col" class="px-5 py-3 font-medium">
                Destination
              </th>
              <th scope="col" class="px-5 py-3 font-medium">
                Title
              </th>
              <th scope="col" class="px-5 py-3 font-medium">
                Status
              </th>
              <th scope="col" class="px-5 py-3 text-right font-medium">
                Clicks
              </th>
              <th scope="col" class="px-5 py-3 font-medium">
                Created
              </th>
              <th scope="col" class="px-5 py-3 font-medium">
                <span class="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="link in items"
              :key="link.id"
              class="border-b border-(--color-border) last:border-0 hover:bg-(--color-surface-muted)"
            >
              <td class="px-5 py-3">
                <div class="flex items-center gap-1">
                  <LinkPreviewCard
                    v-if="previewOf(link)"
                    :preview="previewOf(link)!"
                    :destination-url="link.destination_url"
                    hover
                  >
                    <NuxtLink :to="`/dashboard/links/${link.id}`" class="font-medium hover:underline">
                      {{ link.domain }}/{{ link.slug }}
                    </NuxtLink>
                  </LinkPreviewCard>
                  <NuxtLink v-else :to="`/dashboard/links/${link.id}`" class="font-medium hover:underline">
                    {{ link.domain }}/{{ link.slug }}
                  </NuxtLink>
                  <UiCopyButton :value="link.short_url" label="Copy" />
                </div>
              </td>
              <td class="px-5 py-3 text-(--color-content-muted)">
                <span class="flex items-center gap-2" :title="link.destination_url">
                  <img
                    v-if="previewOf(link)?.favicon_url"
                    :src="previewOf(link)?.favicon_url"
                    alt=""
                    class="size-4 shrink-0 rounded-sm"
                    loading="lazy"
                    referrerpolicy="no-referrer"
                  >
                  {{ truncateMiddle(link.destination_url, 48) }}
                </span>
              </td>
              <td class="px-5 py-3">
                <span v-if="link.title" :title="link.title">{{ truncateMiddle(link.title, 32) }}</span>
                <span v-else class="text-(--color-content-subtle)">—</span>
              </td>
              <td class="px-5 py-3">
                <LinkStatusBadge :status="link.status" />
              </td>
              <td class="px-5 py-3 text-right tabular-nums">
                {{ formatNumber(link.click_count) }}
              </td>
              <td class="whitespace-nowrap px-5 py-3 text-(--color-content-muted)">
                {{ formatDate(link.created_at) }}
              </td>
              <td class="px-5 py-3 text-right">
                <LinkRowActions
                  :link-id="link.id"
                  @copy="copyShortUrl(link)"
                  @qr="showQr(link)"
                  @delete="askDelete(link)"
                />
              </td>
            </tr>
          </tbody>
        </table>

        <div v-if="nextCursor" class="flex justify-center border-t border-(--color-border) px-5 py-4">
          <UiButton variant="secondary" :loading="loadingMore" @click="load(true)">
            Load more
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
      <p v-if="deleteTarget" class="rounded-md bg-(--color-surface-muted) px-3 py-2 font-mono text-xs">
        {{ deleteTarget.short_url }}
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

    <LinkQrModal
      v-if="qrTarget"
      v-model="qrOpen"
      :url="qrTarget.short_url"
      :label="qrTarget.title || `${qrTarget.domain}/${qrTarget.slug}`"
    />
  </div>
</template>
