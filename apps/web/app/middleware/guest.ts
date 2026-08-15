/**
 * For /login and /register: an already-signed-in user has no business there.
 */
export default defineNuxtRouteMiddleware(async () => {
  const services = useServices()
  const session = useSession()

  const setupComplete = useState<boolean | null>('setup.completed', () => null)
  if (setupComplete.value === null) {
    try {
      setupComplete.value = (await services.setup.status()).completed
    } catch {
      setupComplete.value = true
    }
  }

  if (!setupComplete.value) {
    return navigateTo('/setup')
  }

  await session.load()
  if (session.isAuthenticated.value) {
    return navigateTo('/dashboard')
  }
})
