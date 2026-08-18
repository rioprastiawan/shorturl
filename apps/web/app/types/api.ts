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
  language: 'en' | 'id'
  timezone: string
  created_at: string
}

export interface TwoFactorSetup {
  secret: string
  uri: string
  recovery_codes: string[]
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
  two_factor_enabled: boolean
  created_at: string
}

export interface WorkspaceInvitation {
  id: string
  token: string
  role: Exclude<Role, 'owner'>
  expires_at: string
  accepted_at: string | null
  revoked_at: string | null
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
  root_redirect_url: string | null
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
  metadata?: LinkMetadata
  created_via: 'dashboard' | 'api'
  created_by_name: string | null
  created_by_email: string | null
  created_at: string
  updated_at: string
}

export interface PagePreview {
  title?: string
  description?: string
  image_url?: string
  favicon_url?: string
  site_name?: string
  canonical_url?: string
  fetched_at: string
}

export interface LinkMetadata extends Record<string, unknown> {
  preview?: PagePreview
  tags?: string[]
  notes?: string
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
  expiring_links: number
  expiring_api_keys: number
  domain_issues: number
  recent_links: OverviewRecentLink[]
}

export interface OverviewRecentLink {
  id: string
  slug: string
  title: string | null
  short_url: string
  destination_url: string
  status: LinkStatus
  clicks: number
  created_at: string
}

export interface AuditEntry {
  id: string
  action: string
  entity_type: string
  entity_id: string | null
  entity_label: string | null
  actor_name: string | null
  actor_email: string | null
  details: Record<string, unknown>
  created_at: string
}

export interface SystemBranding {
  app_name: string
  organization_name: string
  tagline: string
  login_heading: string
  login_description: string
  footer_text: string
  support_email: string
  support_url: string
  documentation_url: string
  privacy_url: string
  terms_url: string
  primary_color: string
  shell_color: string
  show_powered_by: boolean
  logo_light_url: string
  logo_dark_url: string
  logo_compact_url: string
  favicon_url: string
  qr_foreground: string
  qr_background: string
  qr_style: 'square' | 'rounded' | 'dots' | 'extra-rounded' | 'diamond' | 'classy' | 'classy-rounded' | 'soft' | 'star'
  qr_corner_style: 'square' | 'rounded' | 'circle' | 'dot' | 'leaf'
  qr_margin: number
  qr_size: 512 | 1024 | 2048
  qr_use_logo: boolean
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

export interface ClickSummary {
  total_clicks: number
  unique_visitors: number | null
  previous_clicks: number
  growth_percent: number | null
  average_clicks_per_day: number
}

export type AnalyticsRange = '24h' | '7d' | '30d' | '90d' | '1y' | '5y' | 'custom'

export interface ClicksReport {
  range: AnalyticsRange
  granularity: 'hour' | 'day' | 'week'
  from: string
  to: string
  series: SeriesPoint[]
  summary: ClickSummary
  top_links: TopLink[]
  referrers: ValueStat[]
  utm_sources: ValueStat[]
  utm_mediums: ValueStat[]
  utm_campaigns: ValueStat[]
  devices: ValueStat[]
  browsers: ValueStat[]
  os: ValueStat[]
  countries: ValueStat[]
  hours: ValueStat[]
  weekdays: ValueStat[]
  /** False when domain/link filtering makes workspace dimension rollups inapplicable. */
  breakdowns_scoped: boolean
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
  deployment_mode: 'internal' | 'public'
  registration_enabled: boolean
}
