<script setup lang="ts">
import type { ClicksReport } from '~/types/api'
import type { PresetRange } from '~/components/analytics/types'
import AnalyticsBreakdown from '~/components/analytics/Breakdown.vue'
import AnalyticsChart from '~/components/analytics/Chart.vue'
import AnalyticsRangePicker from '~/components/analytics/RangePicker.vue'
import { formatNumber, truncateMiddle } from '~/components/links/format'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Analytics · ShortURL' })

const ws = useWorkspaces()
const { analytics } = useServices()

const range = ref<PresetRange>('7d')
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
    report.value = await analytics.clicks(workspaceId, { range: range.value })
  } catch (error) {
    loadError.value = error instanceof ApiError ? error.message : 'Could not load analytics.'
    report.value = null
  } finally {
    loading.value = false
  }
}

watch([range, () => ws.activeId.value], () => load(), { immediate: true })

const series = computed(() => report.value?.series ?? [])
const topLinks = computed(() => report.value?.top_links ?? [])
const totalClicks = computed(() => series.value.reduce((acc, point) => acc + point.clicks, 0))

/**
 * Every breakdown in one list so the template can skip empty ones without
 * five near-identical `v-if`s. A dimension is absent, not zeroed, when nothing
 * was recorded — a card reading "no data" five times is worse than no card.
 */
const breakdowns = computed(() => [
  { key: 'referrers', title: 'Referrers', items: report.value?.referrers ?? [], empty: 'Direct / none' },
  { key: 'utm_sources', title: 'UTM sources', items: report.value?.utm_sources ?? [], empty: 'No utm_source' },
  { key: 'devices', title: 'Devices', items: report.value?.devices ?? [], empty: 'Unknown' },
  { key: 'browsers', title: 'Browsers', items: report.value?.browsers ?? [], empty: 'Unknown' },
  { key: 'os', title: 'Operating systems', items: report.value?.os ?? [], empty: 'Unknown' },
].filter(card => card.items.length > 0))

const hasData = computed(() => totalClicks.value > 0 || topLinks.value.length > 0)
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">
          Analytics
        </h1>
        <p class="mt-0.5 text-sm text-(--color-content-muted)">
          Clicks across every link in this workspace.
        </p>
      </div>
      <AnalyticsRangePicker v-model="range" :disabled="loading" />
    </header>

    <p v-if="loading" class="text-sm text-(--color-content-muted)" role="status">
      Loading analytics…
    </p>

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
