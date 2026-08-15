<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { formatDateTime } from '~/components/links/format'

definePageMeta({ middleware: 'auth' })
useHead({ title: 'Activity log · ShortURL' })

const ws = useWorkspaces()
const { links } = useServices()
const page = ref(1)
const pageSize = 25

const { data, pending, error, refresh } = await useAsyncData(
  'workspace-activity-log',
  () => links.auditLog(ws.requireActiveId(), { page: page.value, per_page: pageSize }),
  { watch: [ws.activeId, page] },
)

const entries = computed(() => data.value?.data ?? [])
const total = computed(() => data.value?.meta.total ?? entries.value.length)
const errorMessage = computed(() => error.value instanceof ApiError ? error.value.message : error.value ? 'Could not load activity.' : null)

watch(ws.activeId, () => (page.value = 1))

function actionLabel(action: string) {
  return ({
    'link.created': 'created a link', 'link.updated': 'updated a link',
    'link.active': 'enabled a link', 'link.disabled': 'disabled a link',
    'link.archived': 'archived a link', 'link.deleted': 'deleted a link',
  } as Record<string, string>)[action] ?? action.replaceAll('.', ' ')
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <p class="mb-1 text-sm font-semibold text-(--color-accent)">Workspace</p>
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Activity log</h1>
      <p class="mt-0.5 text-sm text-(--color-content-muted)">A chronological record of link changes in this workspace.</p>
    </header>

    <UiCard :title="`Activity (${total})`" :padded="false">
      <div v-if="pending && !data" role="status"><UiSkeletonTable :rows="8" :columns="3" /></div>
      <div v-else-if="errorMessage" class="px-5 py-8 text-center">
        <p class="text-sm text-(--color-danger)">{{ errorMessage }}</p>
        <UiButton class="mt-3" variant="secondary" size="sm" @click="refresh()">Retry</UiButton>
      </div>
      <UiEmptyState v-else-if="!entries.length" title="No activity yet" description="Link changes will appear here." />
      <ul v-else class="divide-y divide-(--color-border)">
        <li v-for="entry in entries" :key="entry.id" class="flex gap-3 px-5 py-3.5">
          <span class="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full bg-(--color-accent)/10 text-(--color-accent)"><Icon name="lucide:history" size="15" /></span>
          <div class="min-w-0 flex-1">
            <p class="text-sm"><strong>{{ entry.actor_name ?? 'Deleted user' }}</strong> {{ actionLabel(entry.action) }}</p>
            <p class="truncate text-xs text-(--color-content-muted)">{{ entry.entity_label ?? entry.entity_type }}</p>
          </div>
          <time class="shrink-0 text-xs text-(--color-content-subtle)">{{ formatDateTime(entry.created_at) }}</time>
        </li>
      </ul>
      <UiPagination v-model:page="page" :total="total" :page-size="pageSize" label="activities" />
    </UiCard>
  </div>
</template>
