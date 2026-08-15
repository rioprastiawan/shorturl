// Mirrors the Go DTOs. Keep field names in sync with the `json:` tags on the
// server; a mismatch here is a silent undefined at runtime, not a build error.

export interface Envelope<T> {
  data: T
}

export interface ListEnvelope<T> {
  data: T[]
  meta: {
    next_cursor: string | null
    total?: number
  }
}

export interface ApiErrorBody {
  code: string
  message: string
  fields?: Record<string, string[]>
  request_id?: string
}

export interface User {
  id: string
  name: string
  email: string
  is_admin: boolean
  created_at: string
}

export type Role = 'owner' | 'admin' | 'member'

export interface Workspace {
  id: string
  name: string
  slug: string
  role: Role
  created_at: string
}

export interface Member {
  user_id: string
  name: string
  email: string
  role: Role
  created_at: string
}

export type DomainStatus = 'pending' | 'verifying' | 'active' | 'failed' | 'disabled'
export type SslStatus = 'pending' | 'active' | 'failed'

export interface DnsRecord {
  type: string
  name: string
  value: string
}

export interface DnsInstructions {
  verification: DnsRecord
  routing: DnsRecord[]
}

export interface Domain {
  id: string
  hostname: string
  status: DomainStatus
  ssl_status: SslStatus
  is_default: boolean
  verification_method: string
  verification_error: string | null
  verified_at: string | null
  created_at: string
  dns_instructions?: DnsInstructions
}

export type LinkStatus = 'active' | 'disabled' | 'archived'

export interface Link {
  id: string
  short_url: string
  slug: string
  domain: string
  domain_id: string
  destination_url: string
  title: string | null
  status: LinkStatus
  redirect_type: number
  has_password: boolean
  expires_at: string | null
  max_clicks: number | null
  click_count: number
  external_reference: string | null
  metadata?: Record<string, unknown>
  created_via: 'dashboard' | 'api'
  created_at: string
  updated_at: string
}

export interface ApiKey {
  id: string
  name: string
  key_prefix: string
  scopes: string[]
  last_used_at: string | null
  expires_at: string | null
  revoked_at: string | null
  created_at: string
}

export interface CreatedApiKey extends ApiKey {
  key: string
  warning: string
}

export interface Overview {
  total_links: number
  active_links: number
  total_clicks: number
  clicks_today: number
  active_domains: number
  recent_links: Link[]
}

export interface SeriesPoint {
  period: string
  clicks: number
}

export interface TopLink {
  id: string
  slug: string
  title: string | null
  short_url: string
  clicks: number
}

export interface ValueStat {
  value: string
  clicks: number
}

export type AnalyticsRange = '24h' | '7d' | '30d' | '90d' | 'custom'

export interface ClicksReport {
  range: AnalyticsRange
  granularity: 'hour' | 'day' | 'week'
  series: SeriesPoint[]
  top_links: TopLink[]
  referrers: ValueStat[]
  utm_sources: ValueStat[]
  devices: ValueStat[]
  browsers: ValueStat[]
  os: ValueStat[]
}

/**
 * Per-link analytics.
 *
 * Deliberately NOT the same shape as ClicksReport: the dimensional rollups are
 * keyed by workspace, not by link, so referrer/device/browser breakdowns are
 * absent here rather than empty. Producing them per link would mean scanning
 * the raw click log, which is the one query the analytics design rules out.
 */
export interface LinkAnalytics {
  range: AnalyticsRange
  granularity: 'hour' | 'day' | 'week'
  from: string
  to: string
  link: {
    id: string
    slug: string
    title: string | null
    short_url: string
  }
  /** Lifetime total, not scoped to the selected range. */
  total_clicks: number
  clicks_today: number
  series: SeriesPoint[]
}

export interface SetupStatus {
  completed: boolean
}
