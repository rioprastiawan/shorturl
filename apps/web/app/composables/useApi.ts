import type { ApiErrorBody, Envelope, ListEnvelope } from '~/types/api'

/**
 * ApiError carries the server's structured error so forms can show
 * field-level messages instead of a generic failure banner.
 */
export class ApiError extends Error {
  readonly status: number
  readonly code: string
  readonly fields: Record<string, string[]>
  readonly requestId?: string

  constructor(status: number, body: ApiErrorBody) {
    super(body.message || 'Request failed')
    this.name = 'ApiError'
    this.status = status
    this.code = body.code || 'unknown_error'
    this.fields = body.fields ?? {}
    this.requestId = body.request_id
  }

  /** First message for a field, for rendering under an input. */
  field(name: string): string | undefined {
    return this.fields[name]?.[0]
  }

  get isUnauthorized(): boolean {
    return this.status === 401
  }
}

type Query = Record<string, string | number | boolean | undefined | null>

interface RequestOptions {
  method?: 'GET' | 'POST' | 'PATCH' | 'PUT' | 'DELETE'
  body?: unknown
  query?: Query
  headers?: Record<string, string>
}

/**
 * The single entry point for talking to the API.
 *
 * Every call goes through here so that error shape, credential handling, and
 * the 401 redirect are defined once — scattering raw `$fetch` through pages is
 * how those three things drift apart.
 */
export function useApi() {
  const config = useRuntimeConfig()
  const base = config.public.apiBaseUrl

  async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
    const { method = 'GET', body, query, headers = {} } = options

    // Strip undefined/null so `?status=` never appears for an unset filter.
    const params = new URLSearchParams()
    for (const [key, value] of Object.entries(query ?? {})) {
      if (value !== undefined && value !== null && value !== '') {
        params.set(key, String(value))
      }
    }
    const qs = params.toString()
    const url = `${base}${path}${qs ? `?${qs}` : ''}`

    const response = await fetch(url, {
      method,
      // The session is an HttpOnly cookie; without this it is never sent.
      credentials: 'same-origin',
      headers: {
        Accept: 'application/json',
        ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}),
        ...headers,
      },
      body: body !== undefined ? JSON.stringify(body) : undefined,
    })

    if (response.status === 204) {
      return undefined as T
    }

    const text = await response.text()
    const payload = text ? safeParse(text) : null

    if (!response.ok) {
      const errorBody = (payload as { error?: ApiErrorBody } | null)?.error
      throw new ApiError(response.status, errorBody ?? {
        code: 'unknown_error',
        message: `Request failed with status ${response.status}`,
      })
    }

    return payload as T
  }

  return {
    /** GET returning a single resource, unwrapped from {"data": ...}. */
    async get<T>(path: string, query?: Query): Promise<T> {
      const res = await request<Envelope<T>>(path, { query })
      return res.data
    },

    /** GET returning a collection plus its pagination cursor. */
    async list<T>(path: string, query?: Query): Promise<ListEnvelope<T>> {
      return request<ListEnvelope<T>>(path, { query })
    },

    async post<T>(path: string, body?: unknown): Promise<T> {
      const res = await request<Envelope<T>>(path, { method: 'POST', body })
      return res?.data
    },

    async patch<T>(path: string, body?: unknown): Promise<T> {
      const res = await request<Envelope<T>>(path, { method: 'PATCH', body })
      return res?.data
    },

    async del(path: string, query?: Query): Promise<void> {
      await request<void>(path, { method: 'DELETE', query })
    },
  }
}

function safeParse(text: string): unknown {
  try {
    return JSON.parse(text)
  } catch {
    return null
  }
}
