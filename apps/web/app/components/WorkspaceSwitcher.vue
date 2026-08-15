<script setup lang="ts">
const { workspaces, active, select, create } = useWorkspaces()
const toast = useToast()

const open = ref(false)
const creating = ref(false)
const newName = ref('')
const busy = ref(false)
const root = useTemplateRef<HTMLElement>('root')

onClickOutside(root, () => (open.value = false))

function choose(id: string) {
  select(id)
  open.value = false
  // A full reload is the honest way to swap tenants: every page on screen is
  // showing the previous workspace's data, and refetching piecemeal invites
  // a half-switched UI.
  reloadNuxtApp({ path: '/dashboard' })
}

async function submit() {
  const name = newName.value.trim()
  if (!name) return

  busy.value = true
  try {
    await create(name)
    creating.value = false
    open.value = false
    newName.value = ''
    reloadNuxtApp({ path: '/dashboard' })
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not create workspace')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div ref="root" class="relative">
    <button
      type="button"
      class="flex w-full items-center justify-between gap-2 rounded-md border border-(--color-border) px-3 py-2 text-sm transition-colors hover:bg-(--color-surface-muted)"
      :aria-expanded="open"
      aria-haspopup="listbox"
      @click="open = !open"
    >
      <span class="min-w-0 truncate font-medium">{{ active?.name ?? 'No workspace' }}</span>
      <span class="shrink-0 text-xs text-(--color-content-subtle)" aria-hidden="true">▾</span>
    </button>

    <div
      v-if="open"
      class="absolute left-0 right-0 top-full z-20 mt-1 overflow-hidden rounded-md border border-(--color-border) bg-(--color-surface-raised) shadow-lg"
    >
      <ul role="listbox" class="max-h-64 overflow-y-auto py-1">
        <li v-for="workspace in workspaces" :key="workspace.id">
          <button
            type="button"
            role="option"
            :aria-selected="workspace.id === active?.id"
            class="flex w-full items-center justify-between gap-2 px-3 py-2 text-left text-sm transition-colors hover:bg-(--color-surface-muted)"
            @click="choose(workspace.id)"
          >
            <span class="min-w-0 truncate">{{ workspace.name }}</span>
            <span class="shrink-0 text-xs text-(--color-content-subtle)">{{ workspace.role }}</span>
          </button>
        </li>
      </ul>

      <div class="border-t border-(--color-border) p-2">
        <form v-if="creating" class="flex flex-col gap-2" @submit.prevent="submit">
          <input
            v-model="newName"
            placeholder="Workspace name"
            autofocus
            class="w-full rounded-md border border-(--color-border-strong) bg-transparent px-2 py-1.5 text-sm"
          >
          <div class="flex gap-2">
            <UiButton type="submit" size="sm" :loading="busy">
              Create
            </UiButton>
            <UiButton variant="ghost" size="sm" @click="creating = false">
              Cancel
            </UiButton>
          </div>
        </form>
        <button
          v-else
          type="button"
          class="w-full rounded px-2 py-1.5 text-left text-sm text-(--color-content-muted) transition-colors hover:bg-(--color-surface-muted) hover:text-(--color-content)"
          @click="creating = true"
        >
          + New workspace
        </button>
      </div>
    </div>
  </div>
</template>
