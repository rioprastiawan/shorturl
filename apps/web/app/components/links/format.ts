/**
 * Formatting helpers shared by the links, domains and analytics pages.
 *
 * One module so a timestamp never renders two different ways on two screens.
 * `undefined` as the locale means "whatever the browser is set to", which is
 * the right default for a self-hosted tool that stores no locale preference.
 */

const dateTimeFmt = new Intl.DateTimeFormat(undefined, {
  dateStyle: 'medium',
  timeStyle: 'short',
})

const dateFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })

const numberFmt = new Intl.NumberFormat(undefined)

function parse(iso: string | null | undefined): Date | null {
  if (!iso) return null
  const d = new Date(iso)
  return Number.isNaN(d.getTime()) ? null : d
}

/** "15 Aug 2026, 13:20" — when the time of day matters. */
export function formatDateTime(iso: string | null | undefined): string {
  const d = parse(iso)
  return d ? dateTimeFmt.format(d) : '—'
}

/** "15 Aug 2026" — when the time is noise. */
export function formatDate(iso: string | null | undefined): string {
  const d = parse(iso)
  return d ? dateFmt.format(d) : '—'
}

/** Thousands separators, so 12000 clicks does not read as 1200. */
export function formatNumber(value: number | null | undefined): string {
  if (value === null || value === undefined) return '—'
  return numberFmt.format(value)
}

/**
 * Shortens a URL for display while keeping both ends readable — the host at
 * the front and the distinguishing tail at the back. Callers must still put
 * the untruncated value in a `title` attribute.
 */
export function truncateMiddle(value: string, max = 56): string {
  if (value.length <= max) return value
  const head = Math.ceil((max - 1) / 2)
  const tail = Math.floor((max - 1) / 2)
  return `${value.slice(0, head)}…${value.slice(value.length - tail)}`
}
