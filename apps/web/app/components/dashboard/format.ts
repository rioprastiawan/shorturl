/**
 * Shared formatters for the dashboard pages.
 *
 * Kept in one module so a date never renders two different ways on two
 * screens, and so there is a single place to change the locale strategy.
 * `undefined` as the locale means "whatever the browser is set to", which is
 * the right default for a self-hosted tool with no locale preference stored.
 */

const dateTime = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const dateOnly = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

const number = new Intl.NumberFormat(undefined)

/** "15 Aug 2026, 13:20" — for timestamps where the time matters. */
export function formatDateTime(iso: string | null | undefined): string {
  if (!iso) return '—'
  const parsed = new Date(iso)
  if (Number.isNaN(parsed.getTime())) return '—'
  return dateTime.format(parsed)
}

/** "15 Aug 2026" — for joined dates, where the time is noise. */
export function formatDate(iso: string | null | undefined): string {
  if (!iso) return '—'
  const parsed = new Date(iso)
  if (Number.isNaN(parsed.getTime())) return '—'
  return dateOnly.format(parsed)
}

/** Thousands separators, so 12000 clicks does not read as 1200. */
export function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—'
  return number.format(value)
}

/**
 * Shortens a URL for display while keeping both ends readable — the host at
 * the front and the distinguishing path segment at the back.
 */
export function truncateMiddle(value: string, max = 56): string {
  if (value.length <= max) return value
  const head = Math.ceil((max - 1) / 2)
  const tail = Math.floor((max - 1) / 2)
  return `${value.slice(0, head)}…${value.slice(value.length - tail)}`
}
