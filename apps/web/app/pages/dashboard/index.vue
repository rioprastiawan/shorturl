<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { formatNumber, truncateMiddle } from '~/components/dashboard/format'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Overview · ShortURL' })

const ws = useWorkspaces()
const { analytics } = useServices()

const { data: overview, pending, error, refresh } = await useAsyncData(
  'analytics-overview',
  () => analytics.overview(ws.requireActiveId()),
  { watch: [ws.activeId] },
)

interface Stat {
  label: string
  value: string
  tone?: 'default' | 'warning'
}

const stats = computed<Stat[]>(() => {
  const o = overview.value
  if (!o) return []
  return [
    { label: 'Total links', value: formatNumber(o.total_links) },
    { label: 'Active links', value: formatNumber(o.active_links) },
    { label: 'Total clicks', value: formatNumber(o.total_clicks) },
    { label: 'Clicks today', value: formatNumber(o.clicks_today) },
    {
      label: 'Active domains',
      value: formatNumber(o.active_domains),
      // Zero here is not a neutral statistic, it is the thing blocking them.
      tone: o.active_domains === 0 ? 'warning' : 'default',
    },
  ]
})

const needsDomain = computed(() => overview.value?.active_domains === 0)

const errorMessage = computed(() => {
  const e = error.value
  if (!e) return null
  return e instanceof ApiError ? e.message : 'Could not load the overview.'
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">
          Overview
        </h1>
        <p class="text-sm text-(--color-content-muted)">
          {{ ws.active.value?.name ?? 'Workspace' }}
        </p>
      </div>
      <UiButton to="/dashboard/links/new" size="sm">
        New link
      </UiButton>
    </header>

    <p v-if="pending && !overview" class="text-sm text-(--color-content-muted)" role="status">
      Loading…
    </p>

    <UiCard v-else-if="errorMessage" title="Could not load the overview">
      <p class="text-sm text-(--color-content-muted)" role="alert">
        {{ errorMessage }}
      </p>
      <template #actions>
        <UiButton variant="secondary" size="sm" @click="refresh()">
          Retry
        </UiButton>
      </template>
    </UiCard>

    <template v-else-if="overview">
      <!-- The one thing that blocks a fresh install, so it sits above the
           numbers rather than being discovered on the domains page. -->
      <section
        v-if="needsDomain"
        class="flex flex-col gap-3 rounded-lg border border-amber-500/30 bg-amber-500/10 p-5 sm:flex-row sm:items-center sm:justify-between"
      >
        <div class="min-w-0">
          <h2 class="font-medium text-amber-800 dark:text-amber-300">
            Connect a domain to start creating links
          </h2>
          <p class="mt-1 text-sm text-amber-800/80 dark:text-amber-300/80">
            Short links live on a domain you control. Add one and verify it with
            a DNS record — until then, link creation is unavailable.
          </p>
        </div>
        <div class="shrink-0">
          <UiButton to="/dashboard/domains">
            Add a domain
          </UiButton>
        </div>
      </section>

      <section class="grid grid-cols-2 gap-3 lg:grid-cols-5">
        <DashboardStatTile
          v-for="stat in stats"
          :key="stat.label"
          :label="stat.label"
          :value="stat.value"
          :tone="stat.tone"
        />
      </section>

      <UiCard title="Recent links" :padded="false">
        <template #actions>
          <UiButton variant="secondary" size="sm" to="/dashboard/links">
            All links
          </UiButton>
        </template>

        <UiEmptyState
          v-if="!overview.recent_links.length"
          title="No links yet"
          description="Create your first short link and it will show up here."
        >
          <UiButton to="/dashboard/links/new">
            Create a link
          </UiButton>
        </UiEmptyState>

        <ul v-else class="divide-y divide-(--color-border)">
          <li
            v-for="link in overview.recent_links"
            :key="link.id"
            class="flex flex-wrap items-center gap-x-4 gap-y-2 px-5 py-3"
          >
            <div class="min-w-0 flex-1">
              <NuxtLink
                :to="`/dashboard/links/${link.id}`"
                class="font-mono text-sm font-medium hover:underline"
              >
                /{{ link.slug }}
              </NuxtLink>
              <p class="truncate text-xs text-(--color-content-muted)">
                {{ link.short_url }}
              </p>
              <p
                class="mt-0.5 truncate text-xs text-(--color-content-subtle)"
                :title="link.destination_url"
              >
                → {{ truncateMiddle(link.destination_url) }}
              </p>
            </div>

            <div class="flex shrink-0 items-center gap-3">
              <span class="text-sm tabular-nums text-(--color-content-muted)">
                {{ formatNumber(link.click_count) }}
                <span class="text-xs">{{ link.click_count === 1 ? 'click' : 'clicks' }}</span>
              </span>
              <UiCopyButton :value="link.short_url" />
            </div>
          </li>
        </ul>
      </UiCard>
    </template>
  </div>
</template>
