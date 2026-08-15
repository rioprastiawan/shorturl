import type { SeriesPoint, TopLink, ValueStat } from '~/types/api'

/** The four ranges the dashboard offers. `custom` exists server-side but has
 *  no UI yet, so it is deliberately not in this union. */
export type PresetRange = '24h' | '7d' | '30d' | '90d' | '1y' | '5y'

export const RANGE_OPTIONS: { value: PresetRange, label: string }[] = [
  { value: '24h', label: '24h' },
  { value: '7d', label: '7 days' },
  { value: '30d', label: '30 days' },
  { value: '90d', label: '90 days' },
  { value: '1y', label: '1 year' },
  { value: '5y', label: '5 years' },
]

export type Granularity = 'hour' | 'day' | 'week' | 'month'

/**
 * The real shape of `GET /workspaces/{id}/links/{id}/analytics`.
 *
 * The service layer types this endpoint as `ClicksReport`, but the server
 * returns a different DTO: it carries the link, totals and a series, and no
 * dimension breakdowns at all (click_dimension_daily is keyed by workspace,
 * not by link). The breakdown fields are optional here so the page can render
 * them if that ever changes, and so `?? []` is the only defence needed today.
 */
export interface LinkAnalytics {
  range: string
  granularity: Granularity
  from: string
  to: string
  link: TopLink
  total_clicks: number
  clicks_today: number
  series: SeriesPoint[]
  referrers?: ValueStat[]
  utm_sources?: ValueStat[]
  devices?: ValueStat[]
  browsers?: ValueStat[]
  os?: ValueStat[]
}
