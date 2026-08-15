import type { Link } from '~/types/api'

/**
 * The link form's state.
 *
 * Everything is a string because that is what `<input>` and `<select>` hand
 * back; the conversion to the API's numbers, timestamps and nulls happens once
 * at submit time rather than being smeared across the template.
 */
export interface LinkFormModel {
  domain_id: string
  destination_url: string
  slug: string
  title: string
  redirect_type: string
  /** `datetime-local` value: "YYYY-MM-DDTHH:mm" in the browser's zone. */
  expires_at: string
  password: string
  max_clicks: string
  /** Edit only: explicitly clear an existing password. */
  remove_password: boolean
}

export type LinkFormErrors = Partial<Record<keyof LinkFormModel | 'form', string>>

export const REDIRECT_TYPES = [
  { value: '301', label: '301 — Permanent' },
  { value: '302', label: '302 — Temporary' },
  { value: '307', label: '307 — Temporary, keeps the method' },
  { value: '308', label: '308 — Permanent, keeps the method' },
]

export function emptyLinkForm(domainId = ''): LinkFormModel {
  return {
    domain_id: domainId,
    destination_url: '',
    slug: '',
    title: '',
    redirect_type: '302',
    expires_at: '',
    password: '',
    max_clicks: '',
    remove_password: false,
  }
}

/** Pre-fills the edit form. The password is never returned, so it stays blank. */
export function linkToForm(link: Link): LinkFormModel {
  return {
    domain_id: link.domain_id,
    destination_url: link.destination_url,
    slug: link.slug,
    title: link.title ?? '',
    redirect_type: String(link.redirect_type || 302),
    expires_at: toDatetimeLocal(link.expires_at),
    password: '',
    max_clicks: link.max_clicks === null ? '' : String(link.max_clicks),
    remove_password: false,
  }
}

/**
 * RFC3339 → the `datetime-local` shape, in local time.
 *
 * `Date.prototype.toISOString().slice(0, 16)` is the obvious version and it is
 * wrong: it renders UTC into a control the browser reads as local, silently
 * shifting the expiry by the user's offset.
 */
export function toDatetimeLocal(iso: string | null | undefined): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}

/** `datetime-local` → RFC3339, or null when the field is empty or unparseable. */
export function toRfc3339(local: string): string | null {
  if (!local) return null
  const d = new Date(local)
  if (Number.isNaN(d.getTime())) return null
  return d.toISOString()
}

/** Trimmed value, or undefined so the caller can omit the key entirely. */
export function optional(value: string): string | undefined {
  const trimmed = value.trim()
  return trimmed === '' ? undefined : trimmed
}
