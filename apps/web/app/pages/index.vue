<script setup lang="ts">
import { useServices } from '~/services'

/**
 * The root route is a signpost, not a page.
 *
 * Three destinations, in priority order: an unconfigured install has to run
 * the wizard first, a signed-in operator wants the dashboard, and everyone
 * else needs to sign in. `layout: false` because none of the chrome — sidebar,
 * workspace switcher — can render before we know which of those is true.
 */
definePageMeta({ layout: false })

const { setup } = useServices()
const session = useSession()

// Shared with the auth/guest middleware so the status endpoint is hit once per
// page load rather than once per redirect hop.
const setupComplete = useState<boolean | null>('setup.completed', () => null)

onMounted(async () => {
  if (setupComplete.value === null) {
    try {
      setupComplete.value = (await setup.status()).completed
    } catch {
      // The API being unreachable is not the same as setup being incomplete.
      // Assume configured and let /login surface the real connection error.
      setupComplete.value = true
    }
  }

  if (!setupComplete.value) {
    await navigateTo('/setup')
    return
  }

  await session.load()
  await navigateTo(session.isAuthenticated.value ? '/dashboard' : '/login')
})
</script>

<template>
  <div class="grid min-h-dvh place-items-center p-6">
    <div class="flex w-48 flex-col items-center gap-3" role="status" aria-label="Loading application">
      <UiSkeleton height="2.5rem" width="2.5rem" rounded="lg" />
      <UiSkeleton height="0.8rem" width="70%" />
    </div>
  </div>
</template>
