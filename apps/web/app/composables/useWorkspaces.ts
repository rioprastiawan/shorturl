import type { Workspace } from '~/types/api'

const STORAGE_KEY = 'shorturl.activeWorkspace'

/** The selected workspace ID, persisted so a reload keeps your place. */
export function useActiveWorkspaceId() {
  return useState<string | null>('workspaces.activeId', () => {
    if (import.meta.client) {
      return localStorage.getItem(STORAGE_KEY)
    }
    return null
  })
}

export function useWorkspaces() {
  const workspaces = useState<Workspace[]>('workspaces.list', () => [])
  const loaded = useState<boolean>('workspaces.loaded', () => false)
  const activeId = useActiveWorkspaceId()
  const services = useServices()

  const active = computed<Workspace | null>(() => {
    if (!workspaces.value.length) return null
    return workspaces.value.find(w => w.id === activeId.value) ?? workspaces.value[0] ?? null
  })

  async function load(force = false): Promise<Workspace[]> {
    if (loaded.value && !force) return workspaces.value

    const res = await services.workspaces.list()
    workspaces.value = res.data
    loaded.value = true

    // The stored ID can point at a workspace the user was removed from.
    const stillAMember = workspaces.value.some(w => w.id === activeId.value)
    if (!stillAMember) {
      select(workspaces.value[0]?.id ?? null)
    }

    return workspaces.value
  }

  function select(id: string | null) {
    activeId.value = id
    if (import.meta.client) {
      if (id) localStorage.setItem(STORAGE_KEY, id)
      else localStorage.removeItem(STORAGE_KEY)
    }
  }

  async function create(name: string): Promise<Workspace> {
    const workspace = await services.workspaces.create({ name })
    workspaces.value = [...workspaces.value, workspace]
    select(workspace.id)
    return workspace
  }

  /**
   * The active workspace ID, for callers that cannot proceed without one.
   * Throws rather than silently querying `/workspaces/null/...`.
   */
  function requireActiveId(): string {
    const id = active.value?.id
    if (!id) throw new Error('No active workspace')
    return id
  }

  return {
    workspaces: readonly(workspaces),
    loaded: readonly(loaded),
    active,
    activeId: computed(() => active.value?.id ?? null),
    role: computed(() => active.value?.role ?? null),
    load,
    select,
    create,
    requireActiveId,
  }
}
