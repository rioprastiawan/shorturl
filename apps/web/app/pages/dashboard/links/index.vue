<script setup lang="ts">
import type { Domain, Link, PagePreview } from '~/types/api'
import LinkRowActions from '~/components/links/RowActions.vue'
import LinkStatusBadge from '~/components/links/StatusBadge.vue'
import { copyText } from '~/components/links/clipboard'
import { formatDate, formatNumber, truncateMiddle } from '~/components/links/format'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'
import { downloadText, encodeCsv, parseCsv } from '~/utils/csv'

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
const tag = ref('')
const debouncedTag = ref('')

const STATUS_OPTIONS = [
  { value: '', label: 'All statuses' },
  { value: 'active', label: 'Active' },
  { value: 'disabled', label: 'Disabled' },
  { value: 'archived', label: 'Archived' },
]

const LINK_COLUMNS = [
  { key: 'select', label: 'Select', srOnly: true },
  { key: 'short_url', label: 'Short URL' },
  { key: 'destination', label: 'Destination' },
  { key: 'title', label: 'Title' },
  { key: 'tags', label: 'Tags' },
  { key: 'status', label: 'Status' },
  { key: 'clicks', label: 'Clicks', align: 'right' as const },
  { key: 'created', label: 'Created' },
  { key: 'actions', label: 'Actions', align: 'right' as const, srOnly: true },
]

// Debounced so typing "newsletter" is one request, not ten.
let searchTimer: ReturnType<typeof setTimeout> | undefined
let tagTimer: ReturnType<typeof setTimeout> | undefined
watch(search, (value) => {
  clearTimeout(searchTimer)
  searchTimer = setTimeout(() => (debouncedSearch.value = value.trim()), 300)
})
watch(tag, (value) => {
  clearTimeout(tagTimer)
  tagTimer = setTimeout(() => (debouncedTag.value = value.trim().toLowerCase()), 300)
})
onBeforeUnmount(() => {
  clearTimeout(searchTimer)
  clearTimeout(tagTimer)
})

const filtersActive = computed(() =>
  Boolean(debouncedSearch.value || debouncedTag.value || status.value || domainId.value))

function clearFilters() {
  search.value = ''
  debouncedSearch.value = ''
  status.value = ''
  domainId.value = ''
  tag.value = ''
  debouncedTag.value = ''
}

function previewOf(link: Link): PagePreview | null {
  const value = link.metadata?.preview
  if (!value || typeof value !== 'object' || !(value as PagePreview).fetched_at) return null
  return value as PagePreview
}

// ------------------------------------------------------------------ data

const items = ref<Link[]>([])
const nextCursor = ref<string | null>(null)
const cursorHistory = ref<(string | null)[]>([null])
const currentPage = ref(1)
const pending = ref(true)
const loadError = ref<string | null>(null)
const domainOptions = ref<Domain[]>([])
const savedTagOptions = ref<string[]>([])
const selectedIds = ref<string[]>([])
const bulkBusy = ref(false)
const bulkConfirm = ref<'disable' | 'archive' | 'delete' | null>(null)
const bulkTagsOpen = ref(false)
const bulkTags = ref('')
const allPageSelected = computed(() => items.value.length > 0 && items.value.every(item => selectedIds.value.includes(item.id)))
const domainFilterOptions = computed(() => [
  { value: '', label: 'All domains' },
  ...domainOptions.value.map(domain => ({ value: domain.id, label: domain.hostname })),
])

// ---------------------------------------------------------- import / export

interface ImportRow {
  line: number
  values: Record<string, string>
  error?: string
}

interface ImportFailure {
  line: number
  message: string
}

const importInput = ref<HTMLInputElement | null>(null)
const importOpen = ref(false)
const importRows = ref<ImportRow[]>([])
const importFileName = ref('')
const importing = ref(false)
const importedCount = ref(0)
const importFailures = ref<ImportFailure[]>([])
const exporting = ref(false)
const validImportRows = computed(() => importRows.value.filter(row => !row.error))
const importPreviewScrollTop = ref(0)
const IMPORT_PREVIEW_ROW_HEIGHT = 44
const IMPORT_PREVIEW_HEIGHT = 308
const IMPORT_PREVIEW_OVERSCAN = 5
const importPreviewStart = computed(() => Math.max(0, Math.floor(importPreviewScrollTop.value / IMPORT_PREVIEW_ROW_HEIGHT) - IMPORT_PREVIEW_OVERSCAN))
const importPreviewEnd = computed(() => Math.min(
  importRows.value.length,
  Math.ceil((importPreviewScrollTop.value + IMPORT_PREVIEW_HEIGHT) / IMPORT_PREVIEW_ROW_HEIGHT) + IMPORT_PREVIEW_OVERSCAN,
))
const visibleImportRows = computed(() => importRows.value.slice(importPreviewStart.value, importPreviewEnd.value))
const importPreviewTopSpace = computed(() => importPreviewStart.value * IMPORT_PREVIEW_ROW_HEIGHT)
const importPreviewBottomSpace = computed(() => (importRows.value.length - importPreviewEnd.value) * IMPORT_PREVIEW_ROW_HEIGHT)

function openImportDialog() {
  importRows.value = []
  importFileName.value = ''
  importFailures.value = []
  importedCount.value = 0
  importPreviewScrollTop.value = 0
  importOpen.value = true
}

function downloadImportTemplate() {
  downloadText('shorturl-import-template.csv', encodeCsv([
    ['destination_url', 'domain', 'slug', 'title', 'status', 'redirect_type', 'expires_at', 'max_clicks', 'external_reference', 'tags', 'notes'],
    ['https://example.com/campaign', '', 'campaign', 'Campaign link', 'active', 302, '', '', 'campaign-001', 'marketing|campaign', 'Optional notes'],
  ]))
}

function importError(values: Record<string, string>): string | undefined {
  const destinationUrl = values.destination_url ?? ''
  const hostname = values.domain ?? ''
  if (!destinationUrl) return 'destination_url is required'
  try {
    const destination = new URL(destinationUrl)
    if (!['http:', 'https:'].includes(destination.protocol)) return 'destination_url must use http or https'
  } catch { return 'destination_url is not a valid URL' }

  if (hostname && !domainOptions.value.some(domain => domain.hostname.toLowerCase() === hostname.toLowerCase())) {
    return `domain ${hostname} is not active in this workspace`
  }
  if (values.status && !['active', 'disabled', 'archived'].includes(values.status)) return 'status must be active, disabled, or archived'
  if (values.redirect_type && !['301', '302', '307', '308'].includes(values.redirect_type)) return 'redirect_type must be 301, 302, 307, or 308'
  if (values.expires_at && Number.isNaN(Date.parse(values.expires_at))) return 'expires_at must be an ISO date and time'
  if (values.max_clicks) {
    const limit = Number(values.max_clicks)
    if (!Number.isInteger(limit) || limit <= 0) return 'max_clicks must be a positive integer'
  }
}

async function chooseImportFile(event: Event) {
  const input = event.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  await readImportFile(file)
}

async function dropImportFile(event: DragEvent) {
  const file = event.dataTransfer?.files[0]
  if (!file) return
  await readImportFile(file)
}

async function readImportFile(file: File) {
  if (!file.name.toLowerCase().endsWith('.csv') && file.type !== 'text/csv') {
    toast.error('Choose a CSV file to import.')
    return
  }
  if (file.size > 1024 * 1024) {
    toast.error('CSV files are limited to 1 MB.')
    return
  }

  try {
    const parsed = parseCsv(await file.text())
    if (parsed.length < 2) throw new Error('The CSV has no data rows.')
    const headers = parsed[0]!.map(header => header.trim().toLowerCase())
    if (!headers.includes('destination_url')) throw new Error('The CSV must contain a destination_url column.')
    if (parsed.length > 1001) throw new Error('A single import is limited to 1,000 links.')

    importRows.value = parsed.slice(1).map((cells, index) => {
      const values = Object.fromEntries(headers.map((header, cell) => [header, cells[cell]?.trim() ?? '']))
      return { line: index + 2, values, error: importError(values) }
    })
    importFileName.value = file.name
    importFailures.value = []
    importedCount.value = 0
    importPreviewScrollTop.value = 0
  } catch (error) {
    toast.error(error instanceof Error ? error.message : 'Could not read the CSV file.')
  }
}

async function runImport() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || importing.value || !validImportRows.value.length) return
  importing.value = true
  importedCount.value = 0
  importFailures.value = importRows.value.filter(row => row.error).map(row => ({ line: row.line, message: row.error! }))

  for (const row of validImportRows.value) {
    const value = row.values
    const hostname = value.domain ?? ''
    const selectedDomain = hostname
      ? domainOptions.value.find(domain => domain.hostname.toLowerCase() === hostname.toLowerCase())
      : undefined
    const tags = (value.tags ?? '').split('|').map(tag => tag.trim().toLowerCase()).filter(Boolean)
    const metadata: Record<string, unknown> = {}
    if (tags.length) metadata.tags = [...new Set(tags)].slice(0, 8)
    if (value.notes) metadata.notes = value.notes

    try {
      const created = await links.create(workspaceId, {
        destination_url: value.destination_url!,
        domain_id: selectedDomain?.id,
        slug: value.slug || undefined,
        title: value.title || undefined,
        redirect_type: value.redirect_type ? Number(value.redirect_type) : undefined,
        expires_at: value.expires_at ? new Date(value.expires_at).toISOString() : undefined,
        max_clicks: value.max_clicks ? Number(value.max_clicks) : undefined,
        external_reference: value.external_reference || undefined,
        metadata: Object.keys(metadata).length ? metadata : undefined,
      })
      if (value.status && value.status !== 'active') {
        await links.update(workspaceId, created.id, { status: value.status })
      }
      importedCount.value++
    } catch (error) {
      importFailures.value.push({ line: row.line, message: error instanceof ApiError ? error.message : 'Could not create link' })
    }
  }

  importing.value = false
  await Promise.all([load(), loadDomains()])
  if (!importFailures.value.length) toast.success(`${importedCount.value} links imported`)
}

async function exportCsv() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || exporting.value) return
  exporting.value = true
  try {
    const exported: Link[] = []
    let cursor: string | undefined
    do {
      const response = await links.list(workspaceId, {
        search: debouncedSearch.value || undefined,
        tag: debouncedTag.value || undefined,
        status: status.value || undefined,
        domain_id: domainId.value || undefined,
        cursor,
        limit: 100,
      })
      exported.push(...response.data)
      cursor = response.meta?.next_cursor ?? undefined
    } while (cursor)

    const rows: Array<Array<string | number | null | undefined>> = [[
      'short_url', 'domain', 'slug', 'destination_url', 'title', 'status', 'redirect_type',
      'expires_at', 'max_clicks', 'external_reference', 'tags', 'notes', 'click_count', 'created_at',
    ]]
    for (const link of exported) {
      rows.push([
        link.short_url, link.domain, link.slug, link.destination_url, link.title, link.status,
        link.redirect_type, link.expires_at, link.max_clicks, link.external_reference,
        link.metadata?.tags?.join('|'), typeof link.metadata?.notes === 'string' ? link.metadata.notes : '',
        link.click_count, link.created_at,
      ])
    }
    downloadText(`shorturl-links-${new Date().toISOString().slice(0, 10)}.csv`, encodeCsv(rows))
    toast.success(`${exported.length} links exported`)
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not export links.')
  } finally {
    exporting.value = false
  }
}

// A stale response from a filter the user has already changed must not
// overwrite the current one, so every load carries a token.
let requestToken = 0

async function load(cursor: string | null = cursorHistory.value[currentPage.value - 1] ?? null) {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    items.value = []
    pending.value = false
    return
  }

  const token = ++requestToken
  pending.value = true
  loadError.value = null

  try {
    const res = await links.list(workspaceId, {
      search: debouncedSearch.value || undefined,
      tag: debouncedTag.value || undefined,
      status: status.value || undefined,
      domain_id: domainId.value || undefined,
      cursor: cursor ?? undefined,
      limit: 10,
    })
    if (token !== requestToken) return
    items.value = res.data
    selectedIds.value = []
    nextCursor.value = res.meta?.next_cursor ?? null
  } catch (error) {
    if (token !== requestToken) return
    const message = error instanceof ApiError ? error.message : 'Could not load links.'
    loadError.value = message
  } finally {
    if (token === requestToken) {
      pending.value = false
    }
  }
}

function togglePageSelection() {
  selectedIds.value = allPageSelected.value ? [] : items.value.map(item => item.id)
}

async function runBulk(action: 'enable' | 'disable' | 'archive' | 'delete' | 'tags') {
  const workspaceId = ws.activeId.value
  const selected = items.value.filter(item => selectedIds.value.includes(item.id))
  if (!workspaceId || !selected.length || bulkBusy.value) return
  bulkBusy.value = true
  try {
    if (action === 'delete') {
      await Promise.all(selected.map(item => links.remove(workspaceId, item.id)))
    } else if (action === 'tags') {
      const additions = bulkTags.value.split(',').map(value => value.trim().toLowerCase()).filter(Boolean)
      await Promise.all(selected.map(item => links.update(workspaceId, item.id, {
        metadata: {
          ...(item.metadata ?? {}),
          tags: [...new Set([...(item.metadata?.tags ?? []), ...additions])].slice(0, 8),
        },
      })))
    } else {
      const nextStatus = action === 'enable' ? 'active' : action === 'disable' ? 'disabled' : 'archived'
      await Promise.all(selected.map(item => links.update(workspaceId, item.id, { status: nextStatus })))
    }
    toast.success(`${selected.length} link${selected.length === 1 ? '' : 's'} updated`)
    bulkConfirm.value = null
    bulkTagsOpen.value = false
    bulkTags.value = ''
    await load()
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not update the selected links.')
  } finally {
    bulkBusy.value = false
  }
}

function resetPagination() {
  currentPage.value = 1
  cursorHistory.value = [null]
}

async function nextPage() {
  if (!nextCursor.value || pending.value) return
  cursorHistory.value[currentPage.value] = nextCursor.value
  currentPage.value += 1
  await load(nextCursor.value)
}

async function previousPage() {
  if (currentPage.value <= 1 || pending.value) return
  currentPage.value -= 1
  await load(cursorHistory.value[currentPage.value - 1] ?? null)
}

async function loadDomains() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    domainOptions.value = []
    return
  }
  try {
    const [res, savedTags] = await Promise.all([
      domains.list(workspaceId),
      links.tags(workspaceId).catch(() => ({ data: [] as string[] })),
    ])
    domainOptions.value = res.data.filter(d => d.status === 'active')
    savedTagOptions.value = savedTags.data
  } catch {
    // The filter is a convenience; losing it must not break the page.
    domainOptions.value = []
  }
}

watch(() => ws.activeId.value, () => {
  domainId.value = ''
  loadDomains()
}, { immediate: true })

watch([() => ws.activeId.value, debouncedSearch, debouncedTag, status, domainId], () => {
  resetPagination()
  load()
}, {
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
const disableTarget = ref<Link | null>(null)
const togglingId = ref<string | null>(null)

function showQr(link: Link) {
  qrTarget.value = link
  qrOpen.value = true
}

function askDelete(link: Link) {
  deleteTarget.value = link
  deleteOpen.value = true
}

async function requestStatusToggle(link: Link) {
  if (link.status === 'active') {
    disableTarget.value = link
    return
  }
  await setLinkStatus(link, 'active')
}

async function setLinkStatus(link: Link, next: 'active' | 'disabled') {
  const workspaceId = ws.activeId.value
  if (!workspaceId || togglingId.value) return
  togglingId.value = link.id
  try {
    const updated = await links.update(workspaceId, link.id, { status: next })
    const index = items.value.findIndex(item => item.id === link.id)
    if (status.value && status.value !== updated.status) {
      if (index >= 0) items.value.splice(index, 1)
    } else if (index >= 0) {
      items.value[index] = updated
    }
    disableTarget.value = null
    toast.success(next === 'active' ? 'Link enabled' : 'Link disabled')
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not change the link status.')
  } finally {
    togglingId.value = null
  }
}

async function confirmDelete() {
  const target = deleteTarget.value
  const workspaceId = ws.activeId.value
  if (!target || !workspaceId) return

  deleting.value = true
  try {
    await links.remove(workspaceId, target.id)
    if (items.value.length === 1 && currentPage.value > 1) {
      await previousPage()
    } else {
      await load()
    }
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
      <div class="grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto sm:flex-wrap sm:items-center">
        <UiButton variant="secondary" :disabled="importing" @click="openImportDialog">
          <Icon name="lucide:upload" size="16" /> Import CSV
        </UiButton>
        <UiButton variant="secondary" :loading="exporting" @click="exportCsv">
          <Icon name="lucide:download" size="16" /> Export CSV
        </UiButton>
        <UiButton class="col-span-2 sm:col-span-1" @click="createLinkModal.show()">
          <Icon name="lucide:plus" size="17" /> Create short link
        </UiButton>
      </div>
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

        <div class="flex min-w-0 flex-wrap items-center gap-2 border-t border-(--color-border) pt-2.5 sm:flex-nowrap sm:border-l sm:border-t-0 sm:pl-3 sm:pt-0">
          <Icon name="lucide:globe-2" size="15" class="shrink-0 text-(--color-content-subtle)" />
          <UiSelect
            v-model="domainId"
            input-id="filter-domain"
            :options="domainFilterOptions"
            size="sm"
            class="min-w-0 flex-1 basis-36 sm:w-52 sm:flex-none"
          />
          <div class="relative min-w-0 flex-1 basis-32 sm:w-44 sm:flex-none">
            <Icon name="lucide:tag" size="14" class="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-(--color-content-subtle)" />
            <input
              v-model="tag"
              type="search"
              aria-label="Filter by tag"
              placeholder="Filter tag"
              class="h-8 w-full rounded-md border border-(--color-border) bg-(--color-surface-raised) pl-8 pr-2 text-xs outline-none focus:border-(--color-accent)"
            >
          </div>
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
      <div v-if="pending" role="status" aria-label="Loading links">
        <UiSkeletonTable :rows="6" :columns="6" />
      </div>

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

      <div v-if="items.length" class="flex flex-wrap items-center gap-2 border-b border-(--color-border) bg-(--color-surface-muted)/35 px-4 py-2.5">
        <UiButton variant="ghost" size="sm" :disabled="bulkBusy" @click="togglePageSelection">
          {{ allPageSelected ? 'Clear selection' : 'Select page' }}
        </UiButton>
        <template v-if="selectedIds.length">
          <span class="mr-1 text-xs font-semibold text-(--color-content-muted)">{{ selectedIds.length }} selected</span>
          <UiButton variant="secondary" size="sm" :disabled="bulkBusy" @click="runBulk('enable')">Enable</UiButton>
          <UiButton variant="secondary" size="sm" :disabled="bulkBusy" @click="bulkConfirm = 'disable'">Disable</UiButton>
          <UiButton variant="secondary" size="sm" :disabled="bulkBusy" @click="bulkTagsOpen = true">Add tags</UiButton>
          <UiButton variant="secondary" size="sm" :disabled="bulkBusy" @click="bulkConfirm = 'archive'">Archive</UiButton>
          <UiButton variant="ghost" size="sm" :disabled="bulkBusy" @click="bulkConfirm = 'delete'">
            <span class="text-(--color-danger)">Delete</span>
          </UiButton>
        </template>
      </div>

      <UiDataTable v-if="items.length" :columns="LINK_COLUMNS" :rows="items" row-key="id">
        <template #row="{ row: link }">
              <UiTableCell>
                <UiCheckbox v-model="selectedIds" :value="link.id"><span class="sr-only">Select {{ link.short_url }}</span></UiCheckbox>
              </UiTableCell>
              <UiTableCell>
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
              </UiTableCell>
              <UiTableCell muted>
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
              </UiTableCell>
              <UiTableCell>
                <span v-if="link.title" :title="link.title">{{ truncateMiddle(link.title, 32) }}</span>
                <span v-else class="text-(--color-content-subtle)">—</span>
              </UiTableCell>
              <UiTableCell>
                <div v-if="link.metadata?.tags?.length" class="flex max-w-52 flex-wrap gap-1">
                  <button
                    v-for="linkTag in link.metadata.tags.slice(0, 3)"
                    :key="linkTag"
                    type="button"
                    class="rounded-full bg-(--color-accent)/10 px-2 py-0.5 text-[11px] font-medium text-(--color-accent) hover:bg-(--color-accent)/20"
                    @click="tag = linkTag"
                  >
                    {{ linkTag }}
                  </button>
                  <span v-if="link.metadata.tags.length > 3" class="text-[11px] text-(--color-content-muted)">+{{ link.metadata.tags.length - 3 }}</span>
                </div>
                <span v-else class="text-(--color-content-subtle)">—</span>
              </UiTableCell>
              <UiTableCell>
                <LinkStatusBadge :status="link.status" />
              </UiTableCell>
              <UiTableCell align="right" class="tabular-nums">
                {{ formatNumber(link.click_count) }}
              </UiTableCell>
              <UiTableCell muted nowrap>
                {{ formatDate(link.created_at) }}
              </UiTableCell>
              <UiTableCell align="right">
                <LinkRowActions
                  :link-id="link.id"
                  :status="link.status"
                  :disabled="togglingId === link.id"
                  @copy="copyShortUrl(link)"
                  @qr="showQr(link)"
                  @toggle="requestStatusToggle(link)"
                  @delete="askDelete(link)"
                />
              </UiTableCell>
        </template>
        <template #footer>
          <UiCursorPagination
            :page="currentPage"
            :has-next="Boolean(nextCursor)"
            :loading="pending"
            :label="`Showing ${(currentPage - 1) * 10 + 1}–${(currentPage - 1) * 10 + items.length} links`"
            @previous="previousPage"
            @next="nextPage"
          />
        </template>
      </UiDataTable>
    </UiCard>

    <UiModal
      v-model="importOpen"
      title="Import links from CSV"
      :description="importFileName ? `${importFileName} · ${importRows.length} data rows` : 'Upload a CSV file or start with the import template.'"
      size="lg"
    >
      <div class="space-y-4">
        <input ref="importInput" type="file" accept=".csv,text/csv" class="sr-only" @change="chooseImportFile">
        <button
          type="button"
          class="flex w-full flex-col items-center justify-center rounded-xl border-2 border-dashed border-(--color-border-strong) bg-(--color-surface-muted)/45 px-5 py-8 text-center transition-colors hover:border-(--color-accent) hover:bg-(--color-accent)/5"
          @click="importInput?.click()"
          @dragover.prevent
          @drop.prevent="dropImportFile"
        >
          <span class="grid size-11 place-items-center rounded-full bg-(--color-accent)/10 text-(--color-accent)"><Icon name="lucide:file-up" size="21" /></span>
          <span class="mt-3 text-sm font-semibold">{{ importFileName ? 'Choose another CSV file' : 'Choose a CSV file' }}</span>
          <span class="mt-1 text-xs text-(--color-content-muted)">or drag and drop here · maximum 1 MB and 1,000 links</span>
        </button>

        <div class="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-(--color-border) px-3 py-2.5">
          <div><p class="text-sm font-medium">Need the correct format?</p><p class="text-xs text-(--color-content-muted)">Download a ready-to-edit CSV with every supported column.</p></div>
          <UiButton variant="secondary" size="sm" @click="downloadImportTemplate"><Icon name="lucide:download" size="14" /> Download template</UiButton>
        </div>

        <div v-if="importRows.length" class="grid grid-cols-3 gap-3 text-center">
          <div class="rounded-lg bg-(--color-surface-muted) p-3"><p class="text-xl font-bold">{{ importRows.length }}</p><p class="text-xs text-(--color-content-muted)">Rows</p></div>
          <div class="rounded-lg bg-emerald-500/8 p-3"><p class="text-xl font-bold text-(--color-success)">{{ importedCount || validImportRows.length }}</p><p class="text-xs text-(--color-content-muted)">{{ importedCount ? 'Imported' : 'Ready' }}</p></div>
          <div class="rounded-lg bg-red-500/8 p-3"><p class="text-xl font-bold text-(--color-danger)">{{ importFailures.length || importRows.length - validImportRows.length }}</p><p class="text-xs text-(--color-content-muted)">Failed</p></div>
        </div>
        <div v-if="importRows.length" class="overflow-hidden rounded-lg border border-(--color-border)">
          <div class="flex items-center justify-between border-b border-(--color-border) bg-(--color-surface-muted)/60 px-3 py-2">
            <p class="text-sm font-semibold">Import preview</p>
            <p class="text-[10px] text-(--color-content-subtle)">Virtualized · {{ importRows.length }} rows</p>
          </div>
          <div
            class="overflow-auto"
            :style="{ height: `${IMPORT_PREVIEW_HEIGHT}px` }"
            @scroll="importPreviewScrollTop = ($event.currentTarget as HTMLElement).scrollTop"
          >
            <table class="w-full table-fixed text-left text-xs">
              <thead class="sticky top-0 z-10 bg-(--color-surface-raised) shadow-[0_1px_0_var(--color-border)]">
                <tr class="h-9 text-(--color-content-muted)">
                  <th class="w-13 px-3 font-medium">Line</th>
                  <th class="px-3 font-medium">Destination</th>
                  <th class="hidden w-36 px-3 font-medium sm:table-cell">Slug / domain</th>
                  <th class="w-24 px-3 font-medium">Status</th>
                </tr>
              </thead>
              <tbody>
                <tr v-if="importPreviewTopSpace" aria-hidden="true"><td colspan="4" class="p-0" :style="{ height: `${importPreviewTopSpace}px` }" /></tr>
                <tr
                  v-for="row in visibleImportRows"
                  :key="row.line"
                  class="border-b border-(--color-border) last:border-0"
                  :class="row.error ? 'bg-red-500/5' : ''"
                  :style="{ height: `${IMPORT_PREVIEW_ROW_HEIGHT}px` }"
                >
                  <td class="px-3 tabular-nums text-(--color-content-subtle)">{{ row.line }}</td>
                  <td class="px-3"><p class="truncate" :title="row.values.destination_url">{{ row.values.destination_url || '—' }}</p><p v-if="row.error" class="truncate text-[10px] text-(--color-danger)" :title="row.error">{{ row.error }}</p></td>
                  <td class="hidden px-3 sm:table-cell"><p class="truncate">{{ row.values.slug || 'Auto slug' }}</p><p class="truncate text-[10px] text-(--color-content-subtle)">{{ row.values.domain || 'Default domain' }}</p></td>
                  <td class="px-3"><span class="inline-flex max-w-full truncate rounded-full px-2 py-0.5 text-[10px] font-semibold" :class="row.error ? 'bg-red-500/10 text-(--color-danger)' : 'bg-emerald-500/10 text-(--color-success)'">{{ row.error ? 'Invalid' : (row.values.status || 'active') }}</span></td>
                </tr>
                <tr v-if="importPreviewBottomSpace" aria-hidden="true"><td colspan="4" class="p-0" :style="{ height: `${importPreviewBottomSpace}px` }" /></tr>
              </tbody>
            </table>
          </div>
        </div>
        <div v-if="importFailures.length || importRows.some(row => row.error)" class="max-h-56 overflow-y-auto rounded-lg border border-red-500/20 bg-red-500/5 p-3">
          <p class="mb-2 text-sm font-semibold text-(--color-danger)">Rows requiring attention</p>
          <ul class="space-y-1 text-xs">
            <li v-for="failure in (importFailures.length ? importFailures : importRows.filter(row => row.error).map(row => ({ line: row.line, message: row.error! })))" :key="`${failure.line}-${failure.message}`">
              <strong>Line {{ failure.line }}:</strong> {{ failure.message }}
            </li>
          </ul>
        </div>
        <p class="text-xs text-(--color-content-muted)">Required column: <code>destination_url</code>. Optional columns: <code>domain</code>, <code>slug</code>, <code>title</code>, <code>status</code>, <code>redirect_type</code>, <code>expires_at</code>, <code>max_clicks</code>, <code>external_reference</code>, <code>tags</code>, and <code>notes</code>. Separate tags with <code>|</code>.</p>
      </div>
      <template #actions>
        <UiButton variant="secondary" :disabled="importing" @click="importOpen = false">Close</UiButton>
        <UiButton v-if="importRows.length" :loading="importing" :disabled="!validImportRows.length || importedCount > 0" @click="runImport">Import {{ validImportRows.length }} links</UiButton>
      </template>
    </UiModal>

    <UiModal
      :model-value="bulkConfirm !== null"
      :title="bulkConfirm === 'delete' ? 'Delete selected links?' : bulkConfirm === 'archive' ? 'Archive selected links?' : 'Disable selected links?'"
      :description="`${selectedIds.length} selected link${selectedIds.length === 1 ? '' : 's'} will be affected.`"
      :danger="bulkConfirm === 'delete'"
      @update:model-value="open => { if (!open) bulkConfirm = null }"
    >
      <p class="text-sm text-(--color-content-muted)">
        {{ bulkConfirm === 'delete' ? 'Deleted links and their analytics cannot be recovered.' : 'You can change their status again later.' }}
      </p>
      <template #actions>
        <UiButton variant="secondary" :disabled="bulkBusy" @click="bulkConfirm = null">Cancel</UiButton>
        <UiButton
          :variant="bulkConfirm === 'delete' ? 'danger' : 'primary'"
          :loading="bulkBusy"
          @click="bulkConfirm && runBulk(bulkConfirm)"
        >
          {{ bulkConfirm === 'delete' ? 'Delete links' : bulkConfirm === 'archive' ? 'Archive links' : 'Disable links' }}
        </UiButton>
      </template>
    </UiModal>

    <UiModal
      v-model="bulkTagsOpen"
      title="Add tags to selected links"
      :description="`Existing tags are kept for all ${selectedIds.length} selected links.`"
    >
      <LinksTagInput v-model="bulkTags" :suggestions="savedTagOptions" />
      <template #actions>
        <UiButton variant="secondary" :disabled="bulkBusy" @click="bulkTagsOpen = false">Cancel</UiButton>
        <UiButton :loading="bulkBusy" :disabled="!bulkTags" @click="runBulk('tags')">Add tags</UiButton>
      </template>
    </UiModal>

    <UiModal
      :model-value="disableTarget !== null"
      title="Disable this link?"
      description="The short URL will stop redirecting until you enable it again. Its settings and analytics will be kept."
      @update:model-value="open => { if (!open) disableTarget = null }"
    >
      <p v-if="disableTarget" class="rounded-md bg-(--color-surface-muted) px-3 py-2 font-mono text-xs">
        {{ disableTarget.short_url }}
      </p>
      <template #actions>
        <UiButton variant="secondary" :disabled="Boolean(togglingId)" @click="disableTarget = null">
          Cancel
        </UiButton>
        <UiButton
          variant="danger"
          :loading="togglingId === disableTarget?.id"
          @click="disableTarget && setLinkStatus(disableTarget, 'disabled')"
        >
          Disable link
        </UiButton>
      </template>
    </UiModal>

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

    <LinksQrModal
      v-if="qrTarget"
      v-model="qrOpen"
      :url="qrTarget.short_url"
      :label="qrTarget.title || `${qrTarget.domain}/${qrTarget.slug}`"
    />
  </div>
</template>
