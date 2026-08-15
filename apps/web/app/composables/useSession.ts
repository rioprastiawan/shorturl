import type { User } from '~/types/api'
import { ApiError } from './useApi'

/**
 * The authenticated user, resolved once per page load.
 *
 * `useState` rather than Pinia: this is a single value with no mutations
 * beyond "replace it", and the plan asks not to duplicate server state in a
 * store unnecessarily.
 */
export function useSession() {
  const user = useState<User | null>('session.user', () => null)
  const loaded = useState<boolean>('session.loaded', () => false)
  const services = useServices()

  /** Resolve the session from the cookie. Safe to call repeatedly. */
  async function load(force = false): Promise<User | null> {
    if (loaded.value && !force) return user.value

    try {
      user.value = await services.auth.me()
    } catch (error) {
      // A 401 here is the normal "not signed in" case, not a failure.
      if (!(error instanceof ApiError) || !error.isUnauthorized) {
        console.error('Failed to load session', error)
      }
      user.value = null
    } finally {
      loaded.value = true
    }
    return user.value
  }

  function set(next: User | null) {
    user.value = next
    loaded.value = true
  }

  async function logout() {
    try {
      await services.auth.logout()
    } finally {
      // Clear locally even if the request failed: the user asked to leave.
      user.value = null
      loaded.value = true
      useActiveWorkspaceId().value = null
      await navigateTo('/login')
    }
  }

  return {
    user: readonly(user),
    loaded: readonly(loaded),
    isAuthenticated: computed(() => user.value !== null),
    load,
    set,
    logout,
  }
}
