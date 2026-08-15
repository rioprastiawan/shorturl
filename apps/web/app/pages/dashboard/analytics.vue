<script setup lang="ts">
import type { ClicksReport, Domain, Link } from '~/types/api'
import type { CustomRange, PresetRange } from '~/components/analytics/types'
import AnalyticsBreakdown from '~/components/analytics/Breakdown.vue'
import AnalyticsChart from '~/components/analytics/Chart.vue'
import AnalyticsRangePicker from '~/components/analytics/RangePicker.vue'
import { formatNumber, truncateMiddle } from '~/components/links/format'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Analytics · ShortURL' })

const ws = useWorkspaces()
const { analytics, domains, links } = useServices()

const range = ref<PresetRange>('7d')
const customRange = ref<CustomRange | null>(null)
const domainId = ref<string | null>(null)
const linkId = ref<string | null>(null)
const filterDomains = ref<Domain[]>([])
const filterLinks = ref<Link[]>([])
const filtersLoading = ref(false)
const report = ref<ClicksReport | null>(null)
const loading = ref(true)
const loadError = ref<string | null>(null)

async function load() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    loading.value = false
    return
  }
  loading.value = true
  loadError.value = null
  try {
    report.value = await analytics.clicks(workspaceId, {
      range: range.value,
      ...(range.value === 'custom' && customRange.value ? customRange.value : {}),
      domain_id: domainId.value ?? undefined,
      link_id: linkId.value ?? undefined,
    })
  } catch (error) {
    loadError.value = error instanceof ApiError ? error.message : 'Could not load analytics.'
    report.value = null
  } finally {
    loading.value = false
  }
}

watch([range, domainId, linkId, () => ws.activeId.value], () => load(), { immediate: true })

async function loadFilterOptions() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) return
  filtersLoading.value = true
  try {
    const [domainResult, linkResult] = await Promise.all([
      domains.list(workspaceId),
      links.list(workspaceId, { domain_id: domainId.value ?? undefined, limit: 100 }),
    ])
    filterDomains.value = domainResult.data
    filterLinks.value = linkResult.data
  } finally {
    filtersLoading.value = false
  }
}

watch(() => ws.activeId.value, () => {
  domainId.value = null
  linkId.value = null
  loadFilterOptions()
}, { immediate: true })

watch(domainId, () => {
  linkId.value = null
  loadFilterOptions()
})

const domainOptions = computed(() => [
  { value: '', label: 'All domains' },
  ...filterDomains.value.map(domain => ({ value: domain.id, label: domain.hostname })),
])
const linkOptions = computed(() => [
  { value: '', label: 'All links' },
  ...filterLinks.value.map(link => ({ value: link.id, label: `${link.domain}/${link.slug}${link.title ? ` · ${link.title}` : ''}` })),
])
const selectedDomain = computed({ get: () => domainId.value ?? '', set: value => (domainId.value = value || null) })
const selectedLink = computed({ get: () => linkId.value ?? '', set: value => (linkId.value = value || null) })
const hasScopeFilter = computed(() => !!domainId.value || !!linkId.value)

function applyCustomRange(value: CustomRange) {
  customRange.value = value
  if (range.value === 'custom') load()
}

const series = computed(() => report.value?.series ?? [])
const topLinks = computed(() => report.value?.top_links ?? [])
const totalClicks = computed(() => report.value?.summary?.total_clicks
  ?? series.value.reduce((acc, point) => acc + point.clicks, 0))
const peakHour = computed(() => {
  const hours = report.value?.hours ?? []
  return hours.reduce<(typeof hours)[number] | null>(
    (peak, item) => !peak || item.clicks > peak.clicks ? item : peak,
    null,
  )
})
const growth = computed(() => report.value?.summary?.growth_percent ?? null)
const growthLabel = computed(() => {
  if (growth.value === null) return 'No previous-period baseline'
  const prefix = growth.value > 0 ? '+' : ''
  return `${prefix}${growth.value.toFixed(1)}% vs previous period`
})

/**
 * Every breakdown in one list so the template can skip empty ones without
 * five near-identical `v-if`s. A dimension is absent, not zeroed, when nothing
 * was recorded — a card reading "no data" five times is worse than no card.
 */
const breakdowns = computed(() => [
  { key: 'referrers', title: 'Referrers', items: report.value?.referrers ?? [], empty: 'Direct / none' },
  { key: 'utm_sources', title: 'UTM sources', items: report.value?.utm_sources ?? [], empty: 'No utm_source' },
  { key: 'utm_mediums', title: 'UTM mediums', items: report.value?.utm_mediums ?? [], empty: 'No utm_medium' },
  { key: 'utm_campaigns', title: 'UTM campaigns', items: report.value?.utm_campaigns ?? [], empty: 'No utm_campaign' },
  { key: 'browsers', title: 'Browsers', items: report.value?.browsers ?? [], empty: 'Unknown' },
  { key: 'os', title: 'Operating systems', items: report.value?.os ?? [], empty: 'Unknown' },
].filter(card => report.value?.breakdowns_scoped && card.items.length > 0))

const hasData = computed(() => totalClicks.value > 0 || topLinks.value.length > 0)

function csvCell(value: unknown): string {
  const text = String(value ?? '')
  return /[",\n]/.test(text) ? `"${text.replaceAll('"', '""')}"` : text
}

function exportCsv() {
  const current = report.value
  if (!current) return
  const rows: unknown[][] = [
    ['ShortURL analytics export'],
    ['Range', current.range],
    ['From', current.from],
    ['To', current.to],
    ['Domain filter', domainOptions.value.find(item => item.value === selectedDomain.value)?.label ?? 'All domains'],
    ['Link filter', linkOptions.value.find(item => item.value === selectedLink.value)?.label ?? 'All links'],
    [],
    ['Summary'],
    ['Total clicks', current.summary.total_clicks],
    ['Unique visitors', current.summary.unique_visitors],
    ['Previous clicks', current.summary.previous_clicks],
    ['Growth percent', current.summary.growth_percent],
    ['Average clicks per day', current.summary.average_clicks_per_day],
    [],
    ['Clicks over time'],
    ['Period', 'Clicks'],
    ...current.series.map(point => [point.period, point.clicks]),
    [],
    ['Top links'],
    ['Short URL', 'Title', 'Clicks'],
    ...current.top_links.map(link => [link.short_url, link.title, link.clicks]),
  ]
  const dimensions: Array<[string, { value: string, clicks: number }[]]> = [
    ['Referrers', current.referrers], ['UTM sources', current.utm_sources],
    ['UTM mediums', current.utm_mediums], ['UTM campaigns', current.utm_campaigns],
    ['Devices', current.devices], ['Browsers', current.browsers], ['Operating systems', current.os],
    ['Countries', current.countries], ['Hours', current.hours], ['Weekdays', current.weekdays],
  ]
  for (const [title, values] of dimensions) {
    if (!values.length) continue
    rows.push([], [title], ['Value', 'Clicks'], ...values.map(item => [item.value, item.clicks]))
  }
  const csv = rows.map(row => row.map(csvCell).join(',')).join('\n')
  const url = URL.createObjectURL(new Blob([`\uFEFF${csv}`], { type: 'text/csv;charset=utf-8' }))
  const anchor = document.createElement('a')
  anchor.href = url
  anchor.download = `shorturl-analytics-${current.range}-${new Date().toISOString().slice(0, 10)}.csv`
  anchor.click()
  URL.revokeObjectURL(url)
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="mb-1 text-sm font-semibold text-(--color-accent)">Workspace</p>
        <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
          Analytics
        </h1>
        <p class="mt-0.5 text-sm text-(--color-content-muted)">
          Clicks across every link in this workspace.
        </p>
      </div>
      <UiButton variant="secondary" :disabled="loading || !report" @click="exportCsv">
        <Icon name="lucide:download" size="15" /> Export CSV
      </UiButton>
    </header>

    <section class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-3 shadow-sm" aria-label="Analytics filters">
      <div class="grid items-end gap-3 lg:grid-cols-[minmax(10rem,1fr)_minmax(12rem,1.35fr)_auto]">
        <UiSelect v-model="selectedDomain" label="Domain" :options="domainOptions" :disabled="filtersLoading" />
        <UiSelect v-model="selectedLink" label="Link" :options="linkOptions" searchable search-placeholder="Search links…" :disabled="filtersLoading" />
        <div><p class="mb-1 text-sm font-medium">Date range</p><AnalyticsRangePicker v-model="range" :custom-range="customRange" :disabled="loading" @custom="applyCustomRange" /></div>
      </div>
      <div v-if="hasScopeFilter" class="mt-3 flex items-center justify-between gap-3 border-t border-(--color-border) pt-3">
        <p class="text-xs text-(--color-content-muted)">Trend, totals and timing are filtered. Audience breakdowns are hidden because those rollups are workspace-wide.</p>
        <UiButton variant="ghost" size="sm" @click="domainId = null; linkId = null">Reset filters</UiButton>
      </div>
    </section>

    <div v-if="loading" class="flex flex-col gap-4" role="status" aria-label="Loading analytics">
      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5">
        <div v-for="item in 5" :key="item" class="rounded-lg border border-(--color-border) bg-(--color-surface-raised) px-4 py-3">
          <UiSkeleton height="1.7rem" width="45%" /><UiSkeleton class="mt-2" height="0.65rem" width="70%" />
        </div>
      </section>
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-5">
        <UiSkeleton height="1rem" width="9rem" />
        <UiSkeleton class="mt-5" height="19rem" rounded="lg" />
      </div>
      <div class="grid gap-4 lg:grid-cols-2"><UiSkeleton v-for="item in 2" :key="item" height="19rem" rounded="lg" /></div>
    </div>

    <UiCard v-else-if="loadError">
      <p class="text-sm text-(--color-danger)" role="alert">
        {{ loadError }}
      </p>
      <UiButton class="mt-3" variant="secondary" size="sm" @click="load">
        Try again
      </UiButton>
    </UiCard>

    <UiCard v-else-if="!hasData" :padded="false">
      <UiEmptyState
        title="No clicks in this range"
        description="Analytics appear after the first visit to one of your short links. Share a link, then come back — data shows up within a minute."
      >
        <UiButton to="/dashboard/links">
          Go to links
        </UiButton>
      </UiEmptyState>
    </UiCard>

    <template v-else>
      <section class="grid gap-3 sm:grid-cols-2 xl:grid-cols-5" aria-label="Analytics summary">
        <div class="rounded-lg border border-(--color-border) bg-(--color-surface-raised) px-4 py-3">
          <p class="text-2xl font-semibold tabular-nums tracking-tight">
            {{ formatNumber(report?.summary?.total_clicks ?? totalClicks) }}
          </p>
          <p class="mt-0.5 text-xs text-(--color-content-muted)">Clicks in period</p>
        </div>
        <div class="rounded-lg border border-(--color-border) bg-(--color-surface-raised) px-4 py-3">
          <p class="text-2xl font-semibold tabular-nums tracking-tight">
            {{ report?.summary?.unique_visitors == null ? '—' : formatNumber(report.summary.unique_visitors) }}
          </p>
          <p class="mt-0.5 text-xs text-(--color-content-muted)">{{ hasScopeFilter ? 'Unavailable for scoped filters' : 'Unique visitors' }}</p>
        </div>
        <div class="rounded-lg border border-(--color-border) bg-(--color-surface-raised) px-4 py-3">
          <p
            class="text-2xl font-semibold tabular-nums tracking-tight"
            :class="growth !== null && growth < 0 ? 'text-(--color-danger)' : growth !== null && growth > 0 ? 'text-(--color-success)' : ''"
          >
            {{ growth === null ? '—' : `${growth > 0 ? '+' : ''}${growth.toFixed(1)}%` }}
          </p>
          <p class="mt-0.5 text-xs text-(--color-content-muted)">{{ growthLabel }}</p>
        </div>
        <div class="rounded-lg border border-(--color-border) bg-(--color-surface-raised) px-4 py-3">
          <p class="text-2xl font-semibold tabular-nums tracking-tight">
            {{ (report?.summary?.average_clicks_per_day ?? 0).toLocaleString(undefined, { maximumFractionDigits: 1 }) }}
          </p>
          <p class="mt-0.5 text-xs text-(--color-content-muted)">Average clicks/day</p>
        </div>
        <div class="rounded-lg border border-(--color-border) bg-(--color-surface-raised) px-4 py-3">
          <p class="text-2xl font-semibold tabular-nums tracking-tight">
            {{ peakHour?.value ?? '—' }}
          </p>
          <p class="mt-0.5 text-xs text-(--color-content-muted)">
            Peak hour<span v-if="peakHour"> · {{ formatNumber(peakHour.clicks) }} clicks</span>
          </p>
        </div>
      </section>

      <UiCard title="Clicks over time">
        <AnalyticsChart
          v-if="series.length"
          :points="series"
          :granularity="report?.granularity ?? 'day'"
          label="Clicks over time"
        />
        <p v-else class="py-6 text-center text-sm text-(--color-content-muted)">
          No time series for this range.
        </p>
      </UiCard>

      <AnalyticsInsightCharts
        :top-links="topLinks"
        :devices="report?.devices ?? []"
        :hours="report?.hours ?? []"
        :weekdays="report?.weekdays ?? []"
      />

      <AnalyticsCountryMap v-if="report?.breakdowns_scoped && report.countries.length" :countries="report.countries" />

      <UiCard v-if="topLinks.length" title="Top links" :padded="false">
        <div class="overflow-x-auto">
          <table class="w-full min-w-[34rem] text-left text-sm">
            <thead>
              <tr class="border-b border-(--color-border) text-xs uppercase tracking-wide text-(--color-content-subtle)">
                <th scope="col" class="px-5 py-3 font-medium">
                  Short URL
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Title
                </th>
                <th scope="col" class="px-5 py-3 text-right font-medium">
                  Clicks
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="link in topLinks"
                :key="link.id"
                class="border-b border-(--color-border) last:border-0"
              >
                <td class="px-5 py-3">
                  <NuxtLink
                    :to="`/dashboard/links/${link.id}`"
                    class="font-medium hover:underline"
                    :title="link.short_url"
                  >
                    {{ truncateMiddle(link.short_url, 48) }}
                  </NuxtLink>
                </td>
                <td class="px-5 py-3 text-(--color-content-muted)">
                  <span v-if="link.title">{{ truncateMiddle(link.title, 40) }}</span>
                  <span v-else class="text-(--color-content-subtle)">—</span>
                </td>
                <td class="px-5 py-3 text-right tabular-nums">
                  {{ formatNumber(link.clicks) }}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </UiCard>

      <div v-if="breakdowns.length" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <AnalyticsBreakdown
          v-for="card in breakdowns"
          :key="card.key"
          :title="card.title"
          :items="card.items"
          :empty-label="card.empty"
        />
      </div>
    </template>
  </div>
</template>
