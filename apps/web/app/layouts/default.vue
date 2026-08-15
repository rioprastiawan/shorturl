<script setup lang="ts">
const session = useSession()
const { active, load: loadWorkspaces } = useWorkspaces()
const route = useRoute()
const { t } = useUserPreferences()
const createLinkModal = useCreateLinkModal()
const { helpOpen: shortcutHelpOpen, commandOpen } = useDashboardShortcuts()
const moreOpen = ref(false)
const accountMenuOpen = ref(false)
const accountMenu = useTemplateRef<HTMLElement>('accountMenu')
const accountTrigger = useTemplateRef<HTMLElement>('accountTrigger')
const accountPanel = useTemplateRef<HTMLElement>('accountPanel')
const { floatingStyle: accountFloatingStyle } = useFloatingPanel(accountTrigger, accountMenuOpen, {
  width: 'anchor',
  estimatedHeight: 220,
})

onClickOutside([accountMenu, accountPanel], () => (accountMenuOpen.value = false))

await loadWorkspaces()

const groups = computed(() => {
  const role = active.value?.role
  return [
    { label: t('workspace'), items: [
      { to: '/dashboard', label: t('overview'), icon: 'lucide:layout-dashboard', shortcut: 'G O' },
      { to: '/dashboard/links', label: t('links'), icon: 'lucide:link-2', shortcut: 'G L' },
      { to: '/dashboard/analytics', label: t('analytics'), icon: 'lucide:chart-no-axes-combined', shortcut: 'G A' },
    ] },
    { label: t('manage'), items: [
      { to: '/dashboard/domains', label: t('domains'), icon: 'lucide:globe-2', shortcut: 'G D' },
      { to: '/dashboard/members', label: t('members'), icon: 'lucide:users', shortcut: 'G M' },
    ] },
    ...((role === 'owner' || role === 'admin')
      ? [{ label: t('developer'), items: [
          { to: '/dashboard/api-keys', label: t('api'), icon: 'lucide:key-round', shortcut: 'G I' },
          { to: '/dashboard/api-docs', label: t('apiDocs'), icon: 'lucide:book-open-code' },
        ] }]
      : []),
    { label: t('system'), items: [
      { to: '/dashboard/workspace-settings', label: t('settings'), icon: 'lucide:settings', shortcut: 'G S' },
    ] },
  ]
})

const moreItems = computed(() => groups.value.slice(1).flatMap(group => group.items))

function isActive(to: string): boolean {
  if (to === '/dashboard') return route.path === '/dashboard'
  return route.path.startsWith(to)
}

watch(() => route.fullPath, () => {
  moreOpen.value = false
  accountMenuOpen.value = false
})

watch(() => route.query.create, value => {
  if (value === '1') createLinkModal.show()
}, { immediate: true })

async function signOut() {
  moreOpen.value = false
  accountMenuOpen.value = false
  await session.logout()
}
</script>

<template>
  <div class="min-h-dvh bg-(--color-surface-muted) lg:grid lg:grid-cols-[15.5rem_1fr]">
    <header class="sticky top-0 z-30 flex h-14 items-center gap-2 border-b border-(--color-border) bg-(--color-surface-raised)/95 px-3 backdrop-blur sm:px-4 lg:hidden">
      <NuxtLink to="/dashboard" class="flex shrink-0 items-center gap-2.5 font-bold tracking-tight">
        <span class="grid size-8 place-items-center rounded-lg bg-(--color-accent) text-white"><Icon name="lucide:link-2" size="17" /></span>
        <span class="hidden sm:inline">ShortURL</span>
      </NuxtLink>

      <div class="min-w-0 flex-1">
        <WorkspaceSwitcher compact />
      </div>

    </header>

    <Transition name="backdrop-fade">
      <button
        v-if="moreOpen"
        class="fixed inset-0 z-40 bg-slate-950/45 backdrop-blur-sm lg:hidden"
        aria-label="Close more menu"
        @click="moreOpen = false"
      />
    </Transition>

    <aside
      class="mobile-sidebar fixed inset-y-0 left-0 z-50 hidden w-[15.5rem] flex-col bg-[#172033] text-white shadow-2xl dark:bg-[#080c12] lg:sticky lg:top-0 lg:flex lg:h-dvh lg:shadow-none"
    >
      <div class="flex h-15 items-center justify-between px-3.5">
        <NuxtLink to="/dashboard" class="flex items-center gap-3 text-lg font-bold tracking-tight">
          <span class="grid size-9 place-items-center rounded-lg bg-[#84cc16] text-[#172033] shadow-lg shadow-black/10"><Icon name="lucide:link-2" size="19" /></span>
          <span>ShortURL</span>
        </NuxtLink>
      </div>

      <div class="px-3 pb-3">
        <p class="mb-1.5 px-1 text-[10px] font-bold uppercase tracking-[0.16em] text-white/45">{{ t('currentWorkspace') }}</p>
        <WorkspaceSwitcher />
      </div>

      <nav class="flex-1 overflow-y-auto px-2.5 pb-3">
        <div v-for="group in groups" :key="group.label" class="mb-3">
          <p class="mb-1 px-2.5 text-[9px] font-bold uppercase tracking-[0.16em] text-white/40">{{ group.label }}</p>
          <NuxtLink
            v-for="item in group.items"
            :key="item.to"
            :to="item.to"
            class="mb-0.5 flex items-center gap-2.5 rounded-md px-2.5 py-1.5 text-sm font-medium transition-all"
            :class="isActive(item.to) ? 'bg-white text-[#172033] shadow-sm dark:bg-[#202936] dark:text-white' : 'text-white/70 hover:bg-white/8 hover:text-white'"
          >
            <Icon :name="item.icon" size="18" />
            <span class="min-w-0 flex-1">{{ item.label }}</span>
            <kbd
              v-if="item.shortcut"
              class="shrink-0 rounded border px-1.5 py-0.5 font-sans text-[9px] font-semibold tracking-wide"
              :class="isActive(item.to) ? 'border-slate-300 bg-slate-100 text-slate-500 dark:border-white/15 dark:bg-white/8 dark:text-white/50' : 'border-white/12 bg-white/6 text-white/35'"
            >{{ item.shortcut }}</kbd>
          </NuxtLink>
        </div>
      </nav>

      <div class="border-t border-white/10 p-3">
        <div ref="accountMenu" class="relative" @keydown.esc="accountMenuOpen = false">
          <Teleport to="body">
            <Transition name="menu">
              <div
                v-if="accountMenuOpen"
                ref="accountPanel"
                :style="accountFloatingStyle"
                class="overflow-hidden rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-1.5 text-(--color-content) shadow-2xl shadow-black/20"
              >
              <div class="p-2">
                <p class="mb-2 text-[10px] font-bold uppercase tracking-[0.16em] text-(--color-content-subtle)">{{ t('appearance') }}</p>
                <AppearancePicker />
              </div>
              <div class="my-1 border-t border-(--color-border)" />
              <button
                type="button"
                class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors hover:bg-(--color-surface-muted)"
                @click="accountMenuOpen = false; shortcutHelpOpen = true"
              >
                <Icon name="lucide:keyboard" size="17" />
                <span class="flex-1 text-left">Keyboard shortcuts</span>
                <kbd class="rounded border border-(--color-border-strong) bg-(--color-surface-muted) px-1.5 py-0.5 font-sans text-[10px] text-(--color-content-subtle)">?</kbd>
              </button>
              <NuxtLink
                to="/dashboard/account-settings"
                class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium transition-colors hover:bg-(--color-surface-muted)"
                @click="accountMenuOpen = false"
              >
                <Icon name="lucide:user-cog" size="17" />
                <span class="flex-1">{{ t('profile') }}</span>
                <kbd class="rounded border border-(--color-border-strong) bg-(--color-surface-muted) px-1.5 py-0.5 font-sans text-[10px] text-(--color-content-subtle)">G P</kbd>
              </NuxtLink>
              <button
                type="button"
                class="flex w-full items-center gap-2.5 rounded-lg px-2.5 py-2 text-sm font-medium text-(--color-danger) transition-all duration-200 hover:translate-x-0.5 hover:bg-(--color-danger)/8"
                @click="signOut"
              >
                <Icon name="lucide:log-out" size="17" />
                {{ t('signOut') }}
              </button>
              </div>
            </Transition>
          </Teleport>

          <button
            ref="accountTrigger"
            type="button"
            class="flex w-full items-center gap-2.5 rounded-lg bg-white/6 p-2.5 text-left transition-colors hover:bg-white/10"
            :aria-expanded="accountMenuOpen"
            aria-haspopup="menu"
            @click="accountMenuOpen = !accountMenuOpen"
          >
            <span class="grid size-9 shrink-0 place-items-center rounded-full bg-[#84cc16] text-sm font-bold text-[#172033]">{{ session.user.value?.name?.charAt(0).toUpperCase() || 'U' }}</span>
            <span class="min-w-0 flex-1">
              <span class="block truncate text-sm font-semibold">{{ session.user.value?.name }}</span>
              <span class="block truncate text-xs text-white/45">{{ session.user.value?.email }}</span>
            </span>
            <Icon
              name="lucide:chevron-up"
              size="17"
              class="shrink-0 text-white/45 transition-transform"
              :class="accountMenuOpen ? 'rotate-180' : ''"
            />
          </button>
        </div>
      </div>
    </aside>

    <main class="min-w-0 px-3.5 pb-28 pt-3.5 sm:px-4 sm:pt-4 lg:p-5 xl:p-6">
      <div class="dashboard-content mx-auto max-w-7xl"><slot /></div>
    </main>

    <Transition name="bottom-sheet">
      <section
        v-if="moreOpen"
        role="dialog"
        aria-modal="true"
        aria-label="More navigation"
        class="fixed inset-x-0 bottom-0 z-50 rounded-t-3xl border-t border-(--color-border) bg-(--color-surface-raised) px-4 pb-[calc(1rem+env(safe-area-inset-bottom))] pt-3 shadow-2xl lg:hidden"
        @keydown.esc="moreOpen = false"
      >
        <div class="mx-auto mb-3 h-1 w-10 rounded-full bg-(--color-border-strong)" />
        <NuxtLink to="/dashboard/account-settings" class="mb-3 flex items-center gap-3 rounded-xl px-1 py-1 transition-colors hover:bg-(--color-surface-muted)" @click="moreOpen = false">
          <span class="grid size-10 shrink-0 place-items-center rounded-full bg-(--color-accent) text-sm font-bold text-(--color-accent-content)">{{ session.user.value?.name?.charAt(0).toUpperCase() || 'U' }}</span>
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-semibold">{{ session.user.value?.name }}</p>
            <p class="truncate text-xs text-(--color-content-muted)">{{ session.user.value?.email }}</p>
          </div>
          <Icon name="lucide:chevron-right" size="18" class="mr-2 shrink-0 text-(--color-content-subtle)" />
        </NuxtLink>

        <nav class="grid grid-cols-2 gap-2">
          <NuxtLink
            v-for="item in moreItems"
            :key="item.to"
            :to="item.to"
            class="flex items-center gap-3 rounded-xl border border-(--color-border) px-3 py-3 text-sm font-medium transition-colors"
            :class="isActive(item.to) ? 'bg-(--color-accent) text-(--color-accent-content)' : 'bg-(--color-surface-muted) hover:border-(--color-border-strong)'"
          >
            <Icon :name="item.icon" size="18" />
            <span class="truncate">{{ item.label }}</span>
          </NuxtLink>
        </nav>

        <div class="mt-3 border-t border-(--color-border) pt-3">
          <button
            type="button"
            class="mb-2 flex w-full items-center gap-3 rounded-xl px-3 py-2 text-sm font-semibold hover:bg-(--color-surface-muted)"
            @click="moreOpen = false; shortcutHelpOpen = true"
          >
            <Icon name="lucide:keyboard" size="18" /> Keyboard shortcuts
          </button>
          <p class="mb-2 text-[10px] font-bold uppercase tracking-[0.14em] text-(--color-content-subtle)">{{ t('appearance') }}</p>
          <div class="flex items-center justify-between gap-3">
            <AppearancePicker />
            <button type="button" class="flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-semibold text-(--color-danger) hover:bg-(--color-danger)/8" @click="signOut">
              <Icon name="lucide:log-out" size="17" /> {{ t('signOut') }}
            </button>
          </div>
        </div>
      </section>
    </Transition>

    <nav
      class="fixed inset-x-0 bottom-0 z-30 border-t border-(--color-border) bg-(--color-surface-raised)/95 px-2 pt-1.5 shadow-[0_-8px_30px_rgba(15,23,42,0.08)] backdrop-blur lg:hidden"
      style="padding-bottom: max(0.375rem, env(safe-area-inset-bottom));"
      aria-label="Primary navigation"
    >
      <div class="mx-auto grid max-w-md grid-cols-5 items-end">
        <NuxtLink to="/dashboard" class="mobile-bottom-link" :class="isActive('/dashboard') ? 'is-active' : ''">
          <Icon name="lucide:layout-dashboard" size="20" /><span>{{ t('overview') }}</span>
        </NuxtLink>
        <NuxtLink to="/dashboard/links" class="mobile-bottom-link" :class="isActive('/dashboard/links') ? 'is-active' : ''">
          <Icon name="lucide:link-2" size="20" /><span>{{ t('links') }}</span>
        </NuxtLink>
        <button type="button" class="group flex flex-col items-center gap-1 text-[10px] font-semibold text-(--color-content-muted)" aria-label="Create short link" @click="createLinkModal.show()">
          <span class="-mt-5 grid size-13 place-items-center rounded-2xl bg-(--color-accent) text-(--color-accent-content) shadow-lg shadow-(--color-accent)/25 transition-transform group-active:scale-95">
            <Icon name="lucide:plus" size="24" />
          </span>
          <span>Create</span>
        </button>
        <NuxtLink to="/dashboard/analytics" class="mobile-bottom-link" :class="isActive('/dashboard/analytics') ? 'is-active' : ''">
          <Icon name="lucide:chart-no-axes-combined" size="20" /><span>{{ t('analytics') }}</span>
        </NuxtLink>
        <button type="button" class="mobile-bottom-link" :class="moreOpen ? 'is-active' : ''" @click="moreOpen = true">
          <Icon name="lucide:menu" size="20" /><span>More</span>
        </button>
      </div>
    </nav>
    <UiToaster />
    <LinksCreateModal />
    <DashboardCommandPalette v-model="commandOpen" />
    <DashboardShortcutHelp v-model="shortcutHelpOpen" />
  </div>
</template>
