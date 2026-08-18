<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { formatDateTime } from '~/components/links/format'

definePageMeta({ middleware: ['auth', 'workspace-admin'] })
useHead({ title: 'Activity log · ShortURL' })

const ws = useWorkspaces()
const { links } = useServices()
const { tr } = useUserPreferences()
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
  const labels: Record<string, [string, string]> = {
    'link.created': ['created a link', 'membuat tautan'],
    'link.updated': ['updated a link', 'memperbarui tautan'],
    'link.active': ['enabled a link', 'mengaktifkan tautan'],
    'link.disabled': ['disabled a link', 'menonaktifkan tautan'],
    'link.archived': ['archived a link', 'mengarsipkan tautan'],
    'link.deleted': ['deleted a link', 'menghapus tautan'],
    'domain.created': ['added a domain', 'menambahkan domain'],
    'domain.verification_checked': ['checked domain verification', 'memeriksa verifikasi domain'],
    'domain.default_changed': ['changed the default domain', 'mengubah domain default'],
    'domain.root_redirect_updated': ['updated a domain root redirect', 'memperbarui pengalihan root domain'],
    'domain.deleted': ['removed a domain', 'menghapus domain'],
    'workspace.created': ['created the workspace', 'membuat workspace'],
    'workspace.updated': ['renamed the workspace', 'mengganti nama workspace'],
    'member.added': ['added a member', 'menambahkan anggota'],
    'member.role_updated': ['changed a member role', 'mengubah peran anggota'],
    'member.removed': ['removed a member', 'menghapus anggota'],
    'member.left': ['left the workspace', 'meninggalkan workspace'],
    'invitation.created': ['created an invitation', 'membuat undangan'],
    'invitation.renewed': ['renewed an invitation', 'memperbarui undangan'],
    'invitation.revoked': ['revoked an invitation', 'mencabut undangan'],
    'api_key.created': ['created an API key', 'membuat API key'],
    'api_key.revoked': ['revoked an API key', 'mencabut API key'],
  }
  const label = labels[action]
  return label ? tr(label[0], label[1]) : action.replaceAll('.', ' ')
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <p class="mb-1 text-sm font-semibold text-(--color-accent)">Workspace</p>
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Activity log</h1>
      <p class="mt-0.5 text-sm text-(--color-content-muted)">{{ tr('A chronological record of changes in this workspace.', 'Catatan kronologis perubahan di workspace ini.') }}</p>
    </header>

    <UiCard :title="`Activity (${total})`" :padded="false">
      <div v-if="pending && !data" role="status"><UiSkeletonTable :rows="8" :columns="3" /></div>
      <div v-else-if="errorMessage" class="px-5 py-8 text-center">
        <p class="text-sm text-(--color-danger)">{{ errorMessage }}</p>
        <UiButton class="mt-3" variant="secondary" size="sm" @click="refresh()">Retry</UiButton>
      </div>
      <UiEmptyState v-else-if="!entries.length" title="No activity yet" description="Link changes will appear here." />
      <ul v-else class="divide-y divide-(--color-border)">
        <li v-for="entry in entries" :key="entry.id" class="flex flex-wrap gap-3 px-3.5 py-3.5 sm:flex-nowrap sm:px-5">
          <span class="mt-0.5 grid size-8 shrink-0 place-items-center rounded-full bg-(--color-accent)/10 text-(--color-accent)"><Icon name="lucide:history" size="15" /></span>
          <div class="min-w-0 flex-1">
            <p class="text-sm"><strong>{{ entry.actor_name ?? tr('System / API', 'Sistem / API') }}</strong> {{ actionLabel(entry.action) }}</p>
            <p class="truncate text-xs text-(--color-content-muted)">{{ entry.entity_label ?? entry.entity_type }}</p>
          </div>
          <time class="ml-11 shrink-0 text-xs text-(--color-content-subtle) sm:ml-0">{{ formatDateTime(entry.created_at) }}</time>
        </li>
      </ul>
      <UiPagination v-model:page="page" :total="total" :page-size="pageSize" label="activities" />
    </UiCard>
  </div>
</template>
