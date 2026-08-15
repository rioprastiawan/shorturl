<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { formatNumber, truncateMiddle } from '~/components/dashboard/format'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Overview · ShortURL' })

const ws = useWorkspaces()
const session = useSession()
const { analytics } = useServices()
const createLinkModal = useCreateLinkModal()

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
const firstName = computed(() => session.user.value?.name?.trim().split(/\s+/)[0] || 'there')
const onboardingComplete = computed(() => !needsDomain.value && (overview.value?.total_links ?? 0) > 0)

const errorMessage = computed(() => {
  const e = error.value
  if (!e) return null
  return e instanceof ApiError ? e.message : 'Could not load the overview.'
})
</script>

<template>
  <div class="flex flex-col gap-5">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="mb-0.5 text-xs font-semibold text-(--color-accent)">
          {{ ws.active.value?.name ?? 'Workspace' }}
        </p>
        <h1 class="text-2xl font-bold tracking-tight">
          Welcome back, {{ firstName }}
        </h1>
        <p class="mt-1 text-sm text-(--color-content-muted)">
          Here’s what’s happening with your links today.
        </p>
      </div>
      <UiButton @click="createLinkModal.show()">
        <Icon name="lucide:plus" size="17" /> Create short link
      </UiButton>
    </header>

    <div v-if="pending && !overview" class="flex flex-col gap-5" role="status" aria-label="Loading dashboard overview">
      <div class="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <div v-for="item in 4" :key="item" class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-4">
          <UiSkeleton height="0.7rem" width="45%" />
          <UiSkeleton class="mt-3" height="1.8rem" width="35%" />
        </div>
      </div>
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-5">
        <UiSkeleton height="1rem" width="9rem" />
        <div class="mt-5 space-y-4"><UiSkeleton v-for="row in 4" :key="row" height="2.5rem" rounded="lg" /></div>
      </div>
    </div>

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
      <UiCard v-if="!onboardingComplete" compact title="Let’s get your workspace ready" description="Complete these steps to publish and share your first short link.">
        <div class="grid gap-2.5 sm:grid-cols-2">
          <NuxtLink
            to="/dashboard/domains"
            class="flex items-center gap-3 rounded-lg border px-3.5 py-3 transition-colors"
            :class="needsDomain ? 'border-(--color-accent)/30 bg-(--color-accent)/5 hover:bg-(--color-accent)/10' : 'border-emerald-500/20 bg-emerald-500/5'"
          >
            <span class="grid size-8 place-items-center rounded-full" :class="needsDomain ? 'bg-(--color-accent) text-white' : 'bg-emerald-500 text-white'">
              <Icon :name="needsDomain ? 'lucide:globe-2' : 'lucide:check'" size="17" />
            </span>
            <span><strong class="block text-sm">Connect your domain</strong><span class="text-xs text-(--color-content-muted)">{{ needsDomain ? 'Add and verify a domain you control' : 'Domain connected' }}</span></span>
          </NuxtLink>
          <button
            type="button"
            class="flex items-center gap-3 rounded-lg border px-3.5 py-3 transition-colors"
            :class="overview.total_links === 0 ? 'border-(--color-accent)/30 bg-(--color-accent)/5 hover:bg-(--color-accent)/10' : 'border-emerald-500/20 bg-emerald-500/5'"
            @click="createLinkModal.show()"
          >
            <span class="grid size-8 place-items-center rounded-full" :class="overview.total_links === 0 ? 'bg-(--color-accent) text-white' : 'bg-emerald-500 text-white'">
              <Icon :name="overview.total_links === 0 ? 'lucide:link-2' : 'lucide:check'" size="17" />
            </span>
            <span><strong class="block text-sm">Create your first link</strong><span class="text-xs text-(--color-content-muted)">{{ overview.total_links === 0 ? 'Turn a long address into a short link' : 'First link created' }}</span></span>
          </button>
        </div>
      </UiCard>

      <section class="grid grid-cols-2 gap-3 xl:grid-cols-5">
        <DashboardStatTile
          v-for="stat in stats"
          :key="stat.label"
          :label="stat.label"
          :value="stat.value"
          :tone="stat.tone"
        />
      </section>

      <UiCard compact title="Recent links" description="Your latest short links and how they are performing." :padded="false">
        <template #actions>
          <UiButton variant="secondary" size="sm" to="/dashboard/links">
            View all links <Icon name="lucide:arrow-right" size="14" />
          </UiButton>
        </template>

        <UiEmptyState
          v-if="!overview.recent_links.length"
          title="No links yet"
          description="Create your first short link and it will show up here."
        >
          <UiButton @click="createLinkModal.show()">
            Create your first link
          </UiButton>
        </UiEmptyState>

        <ul v-else class="divide-y divide-(--color-border)">
          <li
            v-for="link in overview.recent_links"
            :key="link.id"
            class="flex flex-wrap items-center gap-x-4 gap-y-1.5 px-4 py-2.5 sm:px-5"
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
