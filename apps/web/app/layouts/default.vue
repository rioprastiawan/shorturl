<script setup lang="ts">
const session = useSession()
const { active, load: loadWorkspaces } = useWorkspaces()
const route = useRoute()

const mobileNavOpen = ref(false)

// The layout owns the workspace list because every dashboard page needs it and
// the switcher must be present before any of them render.
await loadWorkspaces()

const nav = computed(() => {
  const role = active.value?.role
  return [
    { to: '/dashboard', label: 'Overview' },
    { to: '/dashboard/links', label: 'Links' },
    { to: '/dashboard/analytics', label: 'Analytics' },
    { to: '/dashboard/domains', label: 'Domains' },
    { to: '/dashboard/members', label: 'Members' },
    // A member cannot create keys, so the page would be a wall of 403s.
    ...(role === 'owner' || role === 'admin'
      ? [{ to: '/dashboard/api-keys', label: 'API keys' }]
      : []),
    { to: '/dashboard/settings', label: 'Settings' },
  ]
})

function isActive(to: string): boolean {
  if (to === '/dashboard') return route.path === '/dashboard'
  return route.path.startsWith(to)
}

watch(() => route.fullPath, () => (mobileNavOpen.value = false))
</script>

<template>
  <div class="min-h-dvh lg:grid lg:grid-cols-[16rem_1fr]">
    <!-- Sidebar -->
    <aside
      class="flex flex-col gap-4 border-b border-(--color-border) bg-(--color-surface-muted) p-4 lg:sticky lg:top-0 lg:h-dvh lg:border-b-0 lg:border-r"
    >
      <div class="flex items-center justify-between">
        <NuxtLink to="/dashboard" class="font-semibold tracking-tight">
          ShortURL
        </NuxtLink>
        <button
          type="button"
          class="rounded-md p-1.5 text-sm lg:hidden"
          :aria-expanded="mobileNavOpen"
          aria-label="Toggle navigation"
          @click="mobileNavOpen = !mobileNavOpen"
        >
          ☰
        </button>
      </div>

      <div :class="mobileNavOpen ? 'flex' : 'hidden'" class="flex-col gap-4 lg:flex">
        <WorkspaceSwitcher />

        <nav class="flex flex-col gap-0.5">
          <NuxtLink
            v-for="item in nav"
            :key="item.to"
            :to="item.to"
            class="rounded-md px-3 py-2 text-sm transition-colors"
            :class="isActive(item.to)
              ? 'bg-(--color-surface-raised) font-medium text-(--color-content) shadow-sm'
              : 'text-(--color-content-muted) hover:bg-(--color-surface-raised) hover:text-(--color-content)'"
            :aria-current="isActive(item.to) ? 'page' : undefined"
          >
            {{ item.label }}
          </NuxtLink>
        </nav>

        <div class="mt-auto flex flex-col gap-2 border-t border-(--color-border) pt-4">
          <div class="px-1">
            <p class="truncate text-sm font-medium">
              {{ session.user.value?.name }}
            </p>
            <p class="truncate text-xs text-(--color-content-muted)">
              {{ session.user.value?.email }}
            </p>
          </div>
          <UiButton variant="ghost" size="sm" @click="session.logout()">
            Sign out
          </UiButton>
        </div>
      </div>
    </aside>

    <main class="min-w-0 p-4 lg:p-8">
      <div class="mx-auto max-w-5xl">
        <slot />
      </div>
    </main>

    <UiToaster />
  </div>
</template>
