<script setup lang="ts">
import { WORKSPACE_SHORTCUT_EVENT } from '~/composables/useDashboardShortcuts'

const props = withDefaults(defineProps<{
  compact?: boolean
  inverse?: boolean
}>(), {
  compact: false,
  inverse: false,
})

const { workspaces, active, select, create } = useWorkspaces()
const toast = useToast()

const open = ref(false)
const creating = ref(false)
const newName = ref('')
const search = ref('')
const busy = ref(false)
const root = useTemplateRef<HTMLElement>('root')
const trigger = useTemplateRef<HTMLElement>('trigger')
const panel = useTemplateRef<HTMLElement>('panel')
const searchInput = useTemplateRef<HTMLInputElement>('searchInput')
const filteredWorkspaces = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  if (!query) return workspaces.value
  return workspaces.value.filter(workspace => workspace.name.toLocaleLowerCase().includes(query))
})
const { floatingStyle } = useFloatingPanel(trigger, open, {
  width: props.compact ? 288 : 'anchor',
  estimatedHeight: 360,
})

onClickOutside([root, panel], () => (open.value = false))

function openFromShortcut() {
  // There is one switcher for mobile and one for desktop; only the currently
  // visible trigger should react to the global shortcut.
  if (trigger.value?.offsetParent === null) return
  open.value = true
}

onMounted(() => window.addEventListener(WORKSPACE_SHORTCUT_EVENT, openFromShortcut))
onBeforeUnmount(() => window.removeEventListener(WORKSPACE_SHORTCUT_EVENT, openFromShortcut))

watch(open, async (isOpen) => {
  if (!isOpen) {
    search.value = ''
    return
  }
  await nextTick()
  searchInput.value?.focus()
})

async function choose(id: string) {
  select(id)
  open.value = false
  // Dashboard pages watch the active workspace and refetch their own data, so
  // this can stay a client-side navigation without reloading the whole app.
  await navigateTo('/dashboard')
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
    await navigateTo('/dashboard')
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
      ref="trigger"
      type="button"
      class="flex w-full items-center justify-between gap-2 rounded-lg border px-2.5 text-sm transition-colors"
      :class="props.compact
        ? props.inverse
          ? 'h-9 border-(--color-shell-border) bg-(--color-shell-hover) text-(--color-shell-content) hover:border-(--color-shell-content-muted)'
          : 'h-9 border-(--color-border) bg-(--color-surface-muted) text-(--color-content) hover:border-(--color-border-strong)'
        : 'border-(--color-shell-border) bg-(--color-shell-hover) py-2 text-(--color-shell-content) hover:border-(--color-shell-content-muted)'"
      :aria-expanded="open"
      aria-haspopup="listbox"
      :aria-label="props.compact ? `Current workspace: ${active?.name ?? 'none'}. Switch workspace` : undefined"
      @click="open = !open"
    >
      <span class="flex min-w-0 items-center gap-2">
        <Icon v-if="props.compact" name="lucide:briefcase-business" size="15" class="shrink-0" :class="props.inverse ? 'text-(--color-shell-content-muted)' : 'text-(--color-content-subtle)'" />
        <span class="min-w-0 truncate font-medium">{{ active?.name ?? 'No workspace' }}</span>
      </span>
      <Icon
        name="lucide:chevron-down"
        size="16"
        class="shrink-0 transition-transform duration-200"
        :class="[
          props.compact && !props.inverse ? 'text-(--color-content-subtle)' : 'text-(--color-shell-content-muted)',
          open ? 'rotate-180' : '',
        ]"
        aria-hidden="true"
      />
    </button>

    <Teleport to="body">
      <Transition name="menu-down">
        <div
          v-if="open"
          ref="panel"
          :style="floatingStyle"
          class="overflow-hidden overflow-y-auto rounded-lg border border-(--color-border) bg-(--color-surface-raised) text-(--color-content) shadow-xl"
        >
      <div class="border-b border-(--color-border) p-2">
        <div class="flex items-center gap-2 rounded-lg border border-(--color-border-strong) bg-(--color-surface-muted) px-2.5">
          <Icon name="lucide:search" size="15" class="shrink-0 text-(--color-content-subtle)" />
          <input
            ref="searchInput"
            v-model="search"
            type="search"
            placeholder="Search workspaces…"
            autocomplete="off"
            class="min-w-0 flex-1 bg-transparent py-2 text-sm outline-none placeholder:text-(--color-content-subtle)"
            @keydown.esc="open = false"
          >
        </div>
      </div>
      <ul role="listbox" class="max-h-64 overflow-y-auto py-1">
        <li v-for="workspace in filteredWorkspaces" :key="workspace.id">
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
        <li v-if="!filteredWorkspaces.length" class="px-3 py-6 text-center text-sm text-(--color-content-muted)">
          No workspaces found
        </li>
      </ul>

      <div class="border-t border-(--color-border) p-2">
        <p class="mb-1 px-2 text-[10px] font-bold uppercase tracking-[0.12em] text-(--color-content-subtle)">
          New workspace
        </p>
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
      <div class="border-t border-(--color-border) p-2">
        <p class="mb-1 px-2 text-[10px] font-bold uppercase tracking-[0.12em] text-(--color-content-subtle)">
          Demo workspace
        </p>
        <WorkspaceDemoCreator block compact @created="open = false" />
      </div>
        </div>
      </Transition>
    </Teleport>
  </div>
</template>
