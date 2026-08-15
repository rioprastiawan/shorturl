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

// ---------------------------------------------------------------- filters

const search = ref('')
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
onBeforeUnmount(() => clearTimeout(searchTimer))

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
  <div class="flex flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">
          Links
        </h1>
        <p class="mt-0.5 text-sm text-(--color-content-muted)">
          Every short link in this workspace.
        </p>
      </div>
      <UiButton to="/dashboard/links/new">
        New link
      </UiButton>
    </header>

    <!-- Filters -->
    <div class="flex flex-wrap items-end gap-3">
      <div class="min-w-[14rem] flex-1">
        <UiInput
          v-model="search"
          label="Search"
          type="search"
          placeholder="Slug, title or destination"
        />
      </div>

      <div class="flex flex-col gap-1.5">
        <label for="filter-status" class="text-sm font-medium">Status</label>
        <select
          id="filter-status"
          v-model="status"
          class="rounded-md border border-(--color-border-strong) bg-transparent px-3 py-2 text-sm"
        >
          <option v-for="option in STATUS_OPTIONS" :key="option.value" :value="option.value">
            {{ option.label }}
          </option>
        </select>
      </div>

      <div class="flex flex-col gap-1.5">
        <label for="filter-domain" class="text-sm font-medium">Domain</label>
        <select
          id="filter-domain"
          v-model="domainId"
          class="rounded-md border border-(--color-border-strong) bg-transparent px-3 py-2 text-sm"
        >
          <option value="">
            All domains
          </option>
          <option v-for="domain in domainOptions" :key="domain.id" :value="domain.id">
            {{ domain.hostname }}
          </option>
        </select>
      </div>

      <UiButton v-if="filtersActive" variant="ghost" @click="clearFilters">
        Clear
      </UiButton>
    </div>

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
        <UiButton to="/dashboard/links/new">
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
  </div>
</template>
