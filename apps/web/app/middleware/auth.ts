/**
 * Gate for every dashboard route.
 *
 * Runs setup detection first: on a fresh install there is no account to sign
 * in with, so sending someone to /login would be a dead end.
 */
export default defineNuxtRouteMiddleware(async (to) => {
  const services = useServices()
  const session = useSession()

  const setupComplete = useState<boolean | null>('setup.completed', () => null)
  if (setupComplete.value === null) {
    try {
      setupComplete.value = (await services.setup.status()).completed
    } catch {
      // If the check itself fails the API is down; let the page render and
      // surface the real error rather than bouncing into a redirect loop.
      setupComplete.value = true
    }
  }

  if (!setupComplete.value) {
    return navigateTo('/setup')
  }

  await session.load()
  if (!session.isAuthenticated.value) {
    return navigateTo({ path: '/login', query: { redirect: to.fullPath } })
  }

  const workspaces = useWorkspaces()
  const items = await workspaces.load()
  if (!items.length && to.path !== '/create-workspace') {
    return navigateTo('/create-workspace')
  }
})
