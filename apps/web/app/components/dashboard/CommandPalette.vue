<script setup lang="ts">
interface CommandItem {
  id: string
  label: string
  description: string
  icon: string
  keywords: string
  run: () => void | Promise<void>
}

const open = defineModel<boolean>({ default: false })
const query = ref('')
const input = useTemplateRef<HTMLInputElement>('input')
const createLinkModal = useCreateLinkModal()
const { workspaces, active, select } = useWorkspaces()
const session = useSession()

async function closeThen(run: () => unknown | Promise<unknown>) {
  open.value = false
  await nextTick()
  await run()
}

async function searchLinks() {
  await navigateTo('/dashboard/links')
  await nextTick()
  requestAnimationFrame(() => document.querySelector<HTMLInputElement>('[data-shortcut-search="links"]')?.focus())
}

const commands = computed<CommandItem[]>(() => [
  {
    id: 'create-link', label: 'Create short link', description: 'Add a new shortened URL',
    icon: 'lucide:circle-plus', keywords: 'new add url short link',
    run: () => closeThen(() => { createLinkModal.show() }),
  },
  {
    id: 'search-links', label: 'Search links', description: 'Find a short link or destination',
    icon: 'lucide:search', keywords: 'find url destination',
    run: () => closeThen(searchLinks),
  },
  ...([
    ['/dashboard', 'Overview', 'lucide:layout-dashboard'],
    ['/dashboard/links', 'Links', 'lucide:link-2'],
    ['/dashboard/analytics', 'Analytics', 'lucide:chart-no-axes-combined'],
    ['/dashboard/appearance', 'Appearance', 'lucide:palette'],
    ['/dashboard/domains', 'Domains', 'lucide:globe-2'],
    ['/dashboard/workspace/members', 'Members', 'lucide:users'],
    ['/dashboard/workspace/activity', 'Activity log', 'lucide:scroll-text'],
    ['/dashboard/api-keys', 'API integrations', 'lucide:key-round'],
    ['/dashboard/workspace/settings', 'Workspace settings', 'lucide:settings'],
    ['/dashboard/account-settings', 'Profile', 'lucide:user-cog'],
  ] as const).map(([to, label, icon]) => ({
    id: `go-${to}`,
    label: `Go to ${label}`,
    description: 'Navigation',
    icon,
    keywords: `navigate page ${label}`,
    run: () => closeThen(async () => { await navigateTo(to) }),
  })),
  ...(session.user.value?.is_admin ? ([
    ['/dashboard/system/settings', 'System settings', 'lucide:settings'],
    ['/dashboard/system/whitelabeling', 'Whitelabeling', 'lucide:badge-check'],
    ['/dashboard/system/qr-branding', 'QR Branding', 'lucide:qr-code'],
  ] as const).map(([to, label, icon]) => ({
    id: `go-${to}`,
    label: `Go to ${label}`,
    description: 'Administration',
    icon,
    keywords: `navigate admin system ${label}`,
    run: () => closeThen(async () => { await navigateTo(to) }),
  })) : []),
  ...workspaces.value.map(workspace => ({
    id: `workspace-${workspace.id}`,
    label: workspace.name,
    description: workspace.id === active.value?.id ? 'Current workspace' : 'Switch workspace',
    icon: 'lucide:briefcase-business',
    keywords: `workspace switch ${workspace.name}`,
    run: () => closeThen(async () => {
      select(workspace.id)
      await navigateTo('/dashboard')
    }),
  })),
])

const filtered = computed(() => {
  const needle = query.value.trim().toLocaleLowerCase()
  if (!needle) return commands.value
  return commands.value.filter(command =>
    `${command.label} ${command.description} ${command.keywords}`.toLocaleLowerCase().includes(needle))
})

watch(open, async (isOpen) => {
  if (!isOpen) {
    query.value = ''
    return
  }
  await nextTick()
  input.value?.focus()
})
</script>

<template>
  <UiModal v-model="open" title="Quick command" description="Search for a page, workspace, or action." size="lg">
    <div class="-mx-1 overflow-hidden rounded-lg border border-(--color-border)">
      <label class="flex items-center gap-3 border-b border-(--color-border) px-4">
        <Icon name="lucide:search" size="19" class="text-(--color-content-subtle)" />
        <input
          ref="input"
          v-model="query"
          type="search"
          autocomplete="off"
          placeholder="Type a command or search…"
          class="min-w-0 flex-1 bg-transparent py-3.5 text-sm outline-none placeholder:text-(--color-content-subtle)"
          @keydown.enter="filtered[0]?.run()"
        >
        <kbd class="rounded border border-(--color-border-strong) bg-(--color-surface-muted) px-1.5 py-0.5 font-sans text-[10px] text-(--color-content-subtle)">ESC</kbd>
      </label>

      <div class="max-h-[min(24rem,55dvh)] overflow-y-auto p-1.5">
        <button
          v-for="command in filtered"
          :key="command.id"
          type="button"
          class="flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left transition-colors hover:bg-(--color-surface-muted) focus:bg-(--color-surface-muted) focus:outline-none"
          @click="command.run()"
        >
          <span class="grid size-7 shrink-0 place-items-center rounded-md bg-(--color-surface-muted)">
            <Icon :name="command.icon" size="17" />
          </span>
          <span class="min-w-0 flex-1">
            <span class="block truncate text-sm font-medium">{{ command.label }}</span>
            <span class="block truncate text-xs text-(--color-content-subtle)">{{ command.description }}</span>
          </span>
          <Icon name="lucide:corner-down-left" size="14" class="text-(--color-content-subtle)" />
        </button>
        <p v-if="!filtered.length" class="px-4 py-10 text-center text-sm text-(--color-content-muted)">No matching commands</p>
      </div>
    </div>
    <template #actions>
      <span class="mr-auto text-xs text-(--color-content-subtle)">Press Enter to run the first result</span>
      <UiButton variant="secondary" @click="open = false">Close</UiButton>
    </template>
  </UiModal>
</template>
