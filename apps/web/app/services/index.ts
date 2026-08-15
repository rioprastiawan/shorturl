import type {
  AnalyticsRange,
  ApiKey,
  ClicksReport,
  CreatedApiKey,
  Domain,
  Link,
  LinkAnalytics,
  Member,
  Overview,
  Role,
  SetupStatus,
  User,
  Workspace,
} from '~/types/api'

/**
 * Typed wrappers around every endpoint, grouped by resource.
 *
 * Pages call these instead of building URLs, so a route change is a one-line
 * edit here rather than a search across components.
 */
export function useServices() {
  const api = useApi()

  const auth = {
    register: (body: { name: string, email: string, password: string }) =>
      api.post<User>('/auth/register', body),
    login: (body: { email: string, password: string }) =>
      api.post<User>('/auth/login', body),
    logout: () => api.post<void>('/auth/logout'),
    me: () => api.get<User>('/auth/me'),
  }

  const setup = {
    status: () => api.get<SetupStatus>('/setup/status'),
    complete: (body: {
      name: string
      email: string
      password: string
      workspace_name: string
    }) => api.post<{ user: User, workspace: Workspace }>('/setup', body),
  }

  const workspaces = {
    list: () => api.list<Workspace>('/workspaces'),
    create: (body: { name: string }) => api.post<Workspace>('/workspaces', body),
    get: (id: string) => api.get<Workspace>(`/workspaces/${id}`),
    update: (id: string, body: { name: string }) =>
      api.patch<Workspace>(`/workspaces/${id}`, body),
    remove: (id: string) => api.del(`/workspaces/${id}`),

    members: (id: string) => api.list<Member>(`/workspaces/${id}/members`),
    addMember: (id: string, body: { email: string, role: Role }) =>
      api.post<Member>(`/workspaces/${id}/members`, body),
    updateMemberRole: (id: string, userId: string, body: { role: Role }) =>
      api.patch<Member>(`/workspaces/${id}/members/${userId}`, body),
    removeMember: (id: string, userId: string) =>
      api.del(`/workspaces/${id}/members/${userId}`),
  }

  const domains = {
    list: (workspaceId: string) => api.list<Domain>(`/workspaces/${workspaceId}/domains`),
    get: (workspaceId: string, id: string) =>
      api.get<Domain>(`/workspaces/${workspaceId}/domains/${id}`),
    create: (workspaceId: string, body: { hostname: string }) =>
      api.post<Domain>(`/workspaces/${workspaceId}/domains`, body),
    verify: (workspaceId: string, id: string) =>
      api.post<Domain>(`/workspaces/${workspaceId}/domains/${id}/verify`),
    setDefault: (workspaceId: string, id: string) =>
      api.post<Domain>(`/workspaces/${workspaceId}/domains/${id}/default`),
    remove: (workspaceId: string, id: string) =>
      api.del(`/workspaces/${workspaceId}/domains/${id}`),
  }

  const links = {
    list: (workspaceId: string, query?: {
      search?: string
      status?: string
      domain_id?: string
      cursor?: string
      limit?: number
    }) => api.list<Link>(`/workspaces/${workspaceId}/links`, query),

    get: (workspaceId: string, id: string) =>
      api.get<Link>(`/workspaces/${workspaceId}/links/${id}`),

    create: (workspaceId: string, body: {
      destination_url: string
      domain_id?: string
      slug?: string
      title?: string
      redirect_type?: number
      password?: string
      expires_at?: string
      max_clicks?: number
    }) => api.post<Link>(`/workspaces/${workspaceId}/links`, body),

    update: (workspaceId: string, id: string, body: Record<string, unknown>) =>
      api.patch<Link>(`/workspaces/${workspaceId}/links/${id}`, body),

    remove: (workspaceId: string, id: string) =>
      api.del(`/workspaces/${workspaceId}/links/${id}`),
  }

  const analytics = {
    overview: (workspaceId: string) =>
      api.get<Overview>(`/workspaces/${workspaceId}/analytics/overview`),
    clicks: (workspaceId: string, query?: { range?: AnalyticsRange, from?: string, to?: string }) =>
      api.get<ClicksReport>(`/workspaces/${workspaceId}/analytics/clicks`, query),
    // Returns LinkAnalytics, not ClicksReport — per-link responses carry
    // lifetime totals and a series, but no dimensional breakdowns.
    forLink: (workspaceId: string, linkId: string, query?: { range?: AnalyticsRange }) =>
      api.get<LinkAnalytics>(`/workspaces/${workspaceId}/links/${linkId}/analytics`, query),
  }

  const apiKeys = {
    list: (workspaceId: string) => api.list<ApiKey>(`/workspaces/${workspaceId}/api-keys`),
    create: (workspaceId: string, body: { name: string, scopes?: string[], test?: boolean }) =>
      api.post<CreatedApiKey>(`/workspaces/${workspaceId}/api-keys`, body),
    revoke: (workspaceId: string, id: string) =>
      api.del(`/workspaces/${workspaceId}/api-keys/${id}`),
  }

  return { auth, setup, workspaces, domains, links, analytics, apiKeys }
}
