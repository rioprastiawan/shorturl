<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { formatDate } from '~/components/dashboard/format'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth', alias: '/dashboard/workspace-settings' })
useHead({ title: 'Workspace Settings · ShortURL' })
const ws = useWorkspaces()
const { workspaces } = useServices()
const toast = useToast()
const isOwner = computed(() => ws.role.value === 'owner')
const name = ref(ws.active.value?.name ?? '')
const renaming = ref(false)
const nameError = ref<string | undefined>()
watch(() => ws.active.value?.name, value => (name.value = value ?? ''))
const renameDirty = computed(() => name.value.trim() !== (ws.active.value?.name ?? ''))

async function rename() {
  const trimmed = name.value.trim()
  nameError.value = undefined
  if (trimmed.length < 2) { nameError.value = 'Must be at least 2 characters'; return }
  renaming.value = true
  try {
    await workspaces.update(ws.requireActiveId(), { name: trimmed })
    await ws.load(true)
    toast.success('Workspace renamed')
  } catch (error) {
    if (error instanceof ApiError) { nameError.value = error.field('name'); if (!nameError.value) toast.error(error.message) }
    else toast.error('Could not rename the workspace')
  } finally { renaming.value = false }
}

const deleteOpen = ref(false)
const deleteConfirmation = ref('')
const deleting = ref(false)
const deleteError = ref<string | null>(null)
const deleteArmed = computed(() => deleteConfirmation.value.trim() === (ws.active.value?.name ?? '__no_workspace__'))
watch(deleteOpen, open => { if (!open) { deleteConfirmation.value = ''; deleteError.value = null } })
async function destroy() {
  if (!deleteArmed.value) return
  deleting.value = true
  deleteError.value = null
  try {
    await workspaces.remove(ws.requireActiveId())
    await ws.load(true)
    deleteOpen.value = false
    toast.success('Workspace deleted')
    await navigateTo('/dashboard')
  } catch (error) {
    if (error instanceof ApiError) deleteError.value = error.code === 'last_workspace' ? 'This is your only workspace, so it cannot be deleted. Create another workspace first.' : error.message
    else deleteError.value = 'Could not delete the workspace.'
  } finally { deleting.value = false }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header><p class="mb-1 text-sm font-semibold text-(--color-accent)">Workspace</p><h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Workspace Settings</h1><p class="text-sm text-(--color-content-muted)">Manage settings for {{ ws.active.value?.name ?? 'the active workspace' }}.</p></header>
    <UiCard title="Workspace details" description="The name shown in the switcher and on shared views."><form class="flex flex-col gap-4" novalidate @submit.prevent="rename"><UiInput v-model="name" label="Workspace name" required :disabled="!isOwner || renaming" :error="nameError" :hint="isOwner ? undefined : 'Only the workspace owner can rename this workspace.'" /><div v-if="isOwner"><UiButton type="submit" :loading="renaming" :disabled="!renameDirty">Save changes</UiButton></div></form><dl class="mt-5 grid gap-4 border-t border-(--color-border) pt-4 text-sm min-[380px]:grid-cols-2"><div><dt class="text-xs text-(--color-content-muted)">Your role</dt><dd class="mt-1 capitalize">{{ ws.role.value ?? '—' }}</dd></div><div><dt class="text-xs text-(--color-content-muted)">Created</dt><dd class="mt-1">{{ formatDate(ws.active.value?.created_at) }}</dd></div></dl></UiCard>
    <UiCard v-if="isOwner" title="Danger zone" description="Irreversible actions for this workspace."><div class="rounded-xl border border-red-500/30 bg-red-500/5 p-4"><h3 class="text-sm font-medium text-(--color-danger)">Delete workspace</h3><p class="mt-1 text-sm text-(--color-content-muted)">Permanently removes its links, domains, members, and click history. Short links stop resolving immediately.</p><div class="mt-3"><UiButton variant="danger" size="sm" @click="deleteOpen = true">Delete workspace</UiButton></div></div></UiCard>
    <UiModal v-model="deleteOpen" title="Delete this workspace?" description="Every link, domain, member, and click record in it is removed. This cannot be undone." danger><div class="flex flex-col gap-3"><DashboardFormAlert v-if="deleteError">{{ deleteError }}</DashboardFormAlert><UiInput v-model="deleteConfirmation" label="Type the workspace name to confirm" :placeholder="ws.active.value?.name" autocomplete="off" /></div><template #actions><UiButton variant="secondary" :disabled="deleting" @click="deleteOpen = false">Cancel</UiButton><UiButton variant="danger" :disabled="!deleteArmed" :loading="deleting" @click="destroy">Delete workspace</UiButton></template></UiModal>
  </div>
</template>
