<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
useHead({ title: 'System Settings · ShortURL' })

const config = useRuntimeConfig()
const webVersion = computed(() => String(config.public.appVersion || 'dev'))
const serverVersion = ref<string | null>(null)
const latestVersion = ref<string | null>(null)
const latestReleaseUrl = ref('https://github.com/rioprastiawan/shorturl/releases')
const checking = ref(true)

function semver(value: string): [number, number, number] | null {
  const match = /(?:^|v)(\d+)\.(\d+)\.(\d+)/.exec(value)
  return match ? [Number(match[1]), Number(match[2]), Number(match[3])] : null
}

const updateAvailable = computed(() => {
  const installed = semver(webVersion.value)
  const latest = semver(latestVersion.value ?? '')
  if (!installed || !latest) return false
  for (let index = 0; index < 3; index++) {
    if (latest[index]! !== installed[index]!) return latest[index]! > installed[index]!
  }
  return false
})

onMounted(async () => {
  const [health, release] = await Promise.allSettled([
    $fetch<{ version: string }>('/api/v1/system/version'),
    $fetch<{ tag_name: string, html_url: string }>('https://api.github.com/repos/rioprastiawan/shorturl/releases/latest'),
  ])
  if (health.status === 'fulfilled') serverVersion.value = health.value.version
  if (release.status === 'fulfilled') {
    latestVersion.value = release.value.tag_name
    latestReleaseUrl.value = release.value.html_url
  }
  checking.value = false
})
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <p class="mb-1 text-sm font-semibold text-(--color-accent)">System</p>
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">System Settings</h1>
      <p class="text-sm text-(--color-content-muted)">Manage this ShortURL installation and check for updates.</p>
    </header>

    <UiCard title="Updates" description="Compare this installation with the latest published release.">
      <div class="flex flex-wrap items-start justify-between gap-4">
        <div>
          <div class="flex flex-wrap items-center gap-2">
            <p class="text-sm font-semibold">Installed {{ webVersion }}</p>
            <UiBadge v-if="updateAvailable" tone="warning" dot>Update available</UiBadge>
            <UiBadge v-else-if="latestVersion" tone="success" dot>Up to date</UiBadge>
            <UiBadge v-else-if="checking" tone="neutral" dot>Checking</UiBadge>
          </div>
          <p v-if="updateAvailable" class="mt-1 text-sm text-(--color-content-muted)">Release {{ latestVersion }} is ready to install.</p>
          <p v-else-if="latestVersion" class="mt-1 text-sm text-(--color-content-muted)">Latest release: {{ latestVersion }}</p>
          <p v-else-if="!checking" class="mt-1 text-sm text-(--color-content-muted)">Could not reach GitHub Releases. Try again later.</p>
        </div>
        <a :href="latestReleaseUrl" target="_blank" rel="noopener noreferrer" class="inline-flex items-center rounded-lg bg-(--color-accent) px-3 py-1.5 text-sm font-semibold text-(--color-accent-content) transition-opacity hover:opacity-90">
          {{ updateAvailable ? 'View update' : 'View releases' }}
        </a>
      </div>
      <p class="mt-4 border-t border-(--color-border) pt-4 text-xs text-(--color-content-muted)">Updates are not installed automatically. Review the release notes, back up persistent data, then deploy the matching image tag in Dokploy or update the checked-out source.</p>
    </UiCard>

    <UiCard title="Build information" description="Versions currently served by each application component.">
      <dl class="grid gap-4 text-sm sm:grid-cols-2">
        <div class="rounded-lg bg-(--color-surface-muted) p-3">
          <dt class="text-xs text-(--color-content-muted)">Dashboard</dt>
          <dd class="mt-1 font-mono font-medium">{{ webVersion }}</dd>
        </div>
        <div class="rounded-lg bg-(--color-surface-muted) p-3">
          <dt class="text-xs text-(--color-content-muted)">Server</dt>
          <dd class="mt-1 font-mono font-medium">{{ serverVersion ?? 'Unavailable' }}</dd>
        </div>
      </dl>
    </UiCard>
  </div>
</template>
