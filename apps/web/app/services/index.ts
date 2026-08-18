import type {
  AuditEntry,
  AnalyticsRange,
  ApiKey,
  ClicksReport,
  CreatedApiKey,
  Domain,
  Link,
  LinkAnalytics,
  PagePreview,
  Member,
  Overview,
  Role,
  SetupStatus,
  SystemBranding,
  TwoFactorSetup,
  User,
  Workspace,
  WorkspaceInvitation,
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
    register: (body: { name: string, email: string, password: string, invitation_token?: string }) =>
      api.post<User>('/auth/register', body),
    login: (body: { email: string, password: string, code?: string }) =>
      api.post<User>('/auth/login', body),
    logout: () => api.post<void>('/auth/logout'),
    me: () => api.get<User>('/auth/me'),
    updateProfile: (body: { name: string, email: string }) =>
      api.patch<User>('/auth/me', body),
    updatePreferences: (body: { language: 'en' | 'id', timezone: string }) =>
      api.patch<User>('/auth/preferences', body),
    changePassword: (body: { current_password: string, new_password: string }) =>
      api.put<void>('/auth/password', body),
    twoFactorStatus: () => api.get<{ enabled: boolean }>('/auth/2fa'),
    twoFactorSetup: (body: { password: string }) => api.post<TwoFactorSetup>('/auth/2fa/setup', body),
    twoFactorEnable: (body: { code: string }) => api.post<void>('/auth/2fa/enable', body),
    twoFactorDisable: (body: { password: string, code: string }) => api.deleteWithBody('/auth/2fa', body),
  }

  const setup = {
    status: () => api.get<SetupStatus>('/setup/status'),
    complete: (body: {
      name: string
      email: string
      password: string
      workspace_name: string
      deployment_mode: 'internal' | 'public'
    }) => api.post<{ user: User, workspace: Workspace }>('/setup', body),
  }

  const branding = {
    get: () => api.get<SystemBranding>('/system/branding'),
    update: (body: SystemBranding) => api.put<SystemBranding>('/system/branding', body),
    removeAsset: (kind: 'logo_light' | 'logo_dark' | 'logo_compact' | 'favicon') =>
      api.del(`/system/branding/assets/${kind}`),
  }

  const workspaces = {
    list: (query?: { search?: string, cursor?: string, limit?: number }) => api.list<Workspace>('/workspaces', query),
    create: (body: { name: string }) => api.post<Workspace>('/workspaces', body),
    createDemo: (body: { size: 'starter' | 'busy' | 'five_year' }) =>
      api.post<Workspace>('/workspaces/demo', body),
    get: (id: string) => api.get<Workspace>(`/workspaces/${id}`),
    update: (id: string, body: { name: string }) =>
      api.patch<Workspace>(`/workspaces/${id}`, body),
    remove: (id: string) => api.del(`/workspaces/${id}`),

    members: (id: string, query?: { page?: number, per_page?: number }) =>
      api.list<Member>(`/workspaces/${id}/members`, query),
    addMember: (id: string, body: { email: string, role: Role }) =>
      api.post<Member>(`/workspaces/${id}/members`, body),
    createInvitation: (id: string, body: { role: Exclude<Role, 'owner'> }) =>
      api.post<WorkspaceInvitation>(`/workspaces/${id}/invitations`, body),
    invitations: (id: string, query?: { page?: number, per_page?: number }) =>
      api.list<WorkspaceInvitation>(`/workspaces/${id}/invitations`, query),
    renewInvitation: (id: string, invitationId: string) =>
      api.post<WorkspaceInvitation>(`/workspaces/${id}/invitations/${invitationId}/renew`),
    revokeInvitation: (id: string, invitationId: string) =>
      api.del(`/workspaces/${id}/invitations/${invitationId}`),
    updateMemberRole: (id: string, userId: string, body: { role: Role }) =>
      api.patch<Member>(`/workspaces/${id}/members/${userId}`, body),
    removeMember: (id: string, userId: string) =>
      api.del(`/workspaces/${id}/members/${userId}`),
  }

  const domains = {
    list: (workspaceId: string, query?: { page?: number, per_page?: number }) =>
      api.list<Domain>(`/workspaces/${workspaceId}/domains`, query),
    get: (workspaceId: string, id: string) =>
      api.get<Domain>(`/workspaces/${workspaceId}/domains/${id}`),
    create: (workspaceId: string, body: { hostname: string }) =>
      api.post<Domain>(`/workspaces/${workspaceId}/domains`, body),
    verify: (workspaceId: string, id: string) =>
      api.post<Domain>(`/workspaces/${workspaceId}/domains/${id}/verify`),
    setDefault: (workspaceId: string, id: string) =>
      api.post<Domain>(`/workspaces/${workspaceId}/domains/${id}/default`),
    updateRootRedirect: (workspaceId: string, id: string, rootRedirectUrl: string | null) =>
      api.patch<Domain>(`/workspaces/${workspaceId}/domains/${id}/root-redirect`, { root_redirect_url: rootRedirectUrl }),
    remove: (workspaceId: string, id: string) =>
      api.del(`/workspaces/${workspaceId}/domains/${id}`),
  }

  const links = {
    list: (workspaceId: string, query?: {
      search?: string
      tag?: string
      status?: string
      domain_id?: string
      cursor?: string
      limit?: number
    }) => api.list<Link>(`/workspaces/${workspaceId}/links`, query),

    get: (workspaceId: string, id: string) =>
      api.get<Link>(`/workspaces/${workspaceId}/links/${id}`),

    tags: (workspaceId: string) =>
      api.list<string>(`/workspaces/${workspaceId}/links/tags`),

    auditLog: (workspaceId: string, query?: { page?: number, per_page?: number }) =>
      api.list<AuditEntry>(`/workspaces/${workspaceId}/links/audit-log`, query),

    create: (workspaceId: string, body: {
      destination_url: string
      domain_id?: string
      slug?: string
      title?: string
      redirect_type?: number
      password?: string
      expires_at?: string
      max_clicks?: number
      external_reference?: string
      metadata?: Record<string, unknown>
    }) => api.post<Link>(`/workspaces/${workspaceId}/links`, body),

    preview: (workspaceId: string, destinationUrl: string) =>
      api.post<PagePreview>(`/workspaces/${workspaceId}/links/preview`, {
        destination_url: destinationUrl,
      }),

    update: (workspaceId: string, id: string, body: Record<string, unknown>) =>
      api.patch<Link>(`/workspaces/${workspaceId}/links/${id}`, body),

    remove: (workspaceId: string, id: string) =>
      api.del(`/workspaces/${workspaceId}/links/${id}`),
  }

  const analytics = {
    overview: (workspaceId: string) =>
      api.get<Overview>(`/workspaces/${workspaceId}/analytics/overview`),
    clicks: (workspaceId: string, query?: {
      range?: AnalyticsRange
      from?: string
      to?: string
      domain_id?: string
      link_id?: string
    }) =>
      api.get<ClicksReport>(`/workspaces/${workspaceId}/analytics/clicks`, query),
    // Returns LinkAnalytics, not ClicksReport — per-link responses carry
    // lifetime totals and a series, but no dimensional breakdowns.
    forLink: (workspaceId: string, linkId: string, query?: { range?: AnalyticsRange, from?: string, to?: string }) =>
      api.get<LinkAnalytics>(`/workspaces/${workspaceId}/links/${linkId}/analytics`, query),
  }

  const apiKeys = {
    list: (workspaceId: string, query?: { cursor?: string, limit?: number }) =>
      api.list<ApiKey>(`/workspaces/${workspaceId}/api-keys`, query),
    create: (workspaceId: string, body: { name: string, scopes?: string[], expires_at?: string, test?: boolean }) =>
      api.post<CreatedApiKey>(`/workspaces/${workspaceId}/api-keys`, body),
    revoke: (workspaceId: string, id: string) =>
      api.del(`/workspaces/${workspaceId}/api-keys/${id}`),
  }

  return { auth, setup, branding, workspaces, domains, links, analytics, apiKeys }
}
