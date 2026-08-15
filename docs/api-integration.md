# API integration guide

How to wire ShortURL into an existing internal system. The machine-readable
contract is [`openapi.yaml`](./openapi.yaml); this document is the practical
version.

Throughout, replace `short.example.com` with your `APP_DOMAIN` and
`shr_live_...` with a real key.

- [Getting a key](#getting-a-key)
- [The 30-second version](#the-30-second-version)
- [The document-sharing pattern](#the-document-sharing-pattern)
- [PHP / Laravel](#php--laravel)
- [Python](#python)
- [Node.js](#nodejs)
- [Error handling](#error-handling)
- [Rate limits](#rate-limits)
- [Security: a short URL is not an authorization boundary](#security-a-short-url-is-not-an-authorization-boundary)

## Getting a key

In the dashboard, go to **API keys** (`/dashboard/api-keys`) and create one. You
need the **admin** or **owner** role in the workspace; a member cannot issue
keys.

**The key is shown exactly once.** Only a lookup prefix and a SHA-256 digest are
stored, so there is no "show key again" button and no support path to recover
it. Copy it straight into your secret store. If you lose it, revoke it and issue
a new one.

Two prefixes exist:

| Prefix | Meaning |
|---|---|
| `shr_live_` | The default. |
| `shr_test_` | Chosen at creation time with `"test": true`. |

**They behave identically on the server.** A `shr_test_` key is not a sandbox:
links it creates are real links on a real domain that real people can click. The
prefix exists so that when a call shows up in *your* logs you can tell staging
traffic from production traffic without holding the key itself. Use two
workspaces, or two domains, if you want genuine separation.

A key belongs to exactly one workspace and carries a fixed set of scopes chosen
at creation:

| Scope | Needed for |
|---|---|
| `links:read` | `GET /links`, `GET /links/{id}`, `GET /links/by-reference/{ref}` |
| `links:write` | `POST /links`, `PATCH /links/{id}`, `DELETE /links/{id}` |

The default is both. Scopes cannot be widened later — a key that turns out to be
too narrow has to be replaced. If your integration only ever creates links, a
`links:write`-only key is a reasonable reduction in blast radius.

Revocation takes effect on the very next request; nothing is cached.

Store the key as an environment variable, never in source control:

```env
SHORTURL_BASE_URL=https://short.example.com/api/v1
SHORTURL_API_KEY=shr_live_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

## The 30-second version

```bash
curl -sS -X POST "https://short.example.com/api/v1/links" \
  -H "Authorization: Bearer $SHORTURL_API_KEY" \
  -H "Content-Type: application/json" \
  -d '{"destination_url": "https://docs.internal.example.com/documents/0198c7e2?token=eyJhbGciOi"}'
```

```json
{
  "data": {
    "id": "0198c7e2-4f1a-7c3d-9b21-5f0a2c8e1d44",
    "short_url": "https://short.example.com/aB7kP2q",
    "slug": "aB7kP2q",
    "domain": "short.example.com",
    "domain_id": "4b6e3c1a-9d52-4d0a-8f77-2c1e5b9a7d31",
    "destination_url": "https://docs.internal.example.com/documents/0198c7e2?token=eyJhbGciOi",
    "title": null,
    "status": "active",
    "redirect_type": 302,
    "has_password": false,
    "expires_at": null,
    "max_clicks": null,
    "click_count": 0,
    "external_reference": null,
    "created_via": "api",
    "created_at": "2026-08-15T10:02:41.117Z",
    "updated_at": "2026-08-15T10:02:41.117Z"
  }
}
```

**`data.short_url` is the field you want.** Everything else is bookkeeping. Put
that string in the WhatsApp message, the email, the SMS, the QR code.

`destination_url` is the only required field. The workspace comes from the key,
the domain defaults to the workspace's default domain, and the slug is generated
randomly from a 7-character alphabet that leaves out `0/O` and `1/l/I` so it
survives being read aloud or retyped from a printout.

Note the envelope: every success is `{"data": …}` and every failure is
`{"error": {"code": …}}`. Never read a top-level field directly.

## The document-sharing pattern

This is what the API is designed around. An internal system has a long,
token-bearing document URL, a user pressed **Share**, and the URL needs to be
short enough to send over WhatsApp.

Set **both** `external_reference` and `Idempotency-Key` to the same
caller-owned identifier — `document:<id>`:

```bash
DOC_ID=0198c7e2-4f1a-7c3d-9b21-5f0a2c8e1d44

curl -sS -X POST "https://short.example.com/api/v1/links" \
  -H "Authorization: Bearer $SHORTURL_API_KEY" \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: document:$DOC_ID" \
  -d "{
        \"destination_url\": \"https://docs.internal.example.com/documents/$DOC_ID?token=eyJhbGciOi\",
        \"external_reference\": \"document:$DOC_ID\",
        \"title\": \"Q3 Handover Pack\"
      }"
```

They do two different jobs, and you want both.

**`Idempotency-Key` makes the retry safe — in the moment.** Your HTTP client
times out, your job runner retries, a user double-clicks Share. Without the
header each attempt creates another short URL pointing at the same document, and
you have no way to tell which one you already sent. With it:

- First request with the key → link created, **201**.
- Same key, same body → the *original* link comes back, **200**. Nothing new was
  created.
- Same key, a *different* body → **409 `idempotency_conflict`**, nothing
  created.

"Same body" is decided by hashing the JSON after canonicalising it, so key order
and whitespace do not matter — including inside `metadata`, which is important
because most languages give you no control over the order their encoder emits
map keys. Values and types do matter: `{"max_clicks": 1}` and
`{"max_clicks": "1"}` are different requests.

Records live for **24 hours**. Beyond that the key is forgotten and the same
request will create a second link, so this protects an in-flight retry, not a
re-share next week.

**`external_reference` makes the link findable later — forever.** It is unique
per workspace, and it means you do not have to add a `short_url` column to your
documents table and keep it in sync. Ask the shortener:

```bash
curl -sS "https://short.example.com/api/v1/links/by-reference/document:$DOC_ID" \
  -H "Authorization: Bearer $SHORTURL_API_KEY"
```

`200` with the link, or `404` if you have never shortened this document. That
one call replaces a whole mapping table.

It is also a second safety net past the 24-hour window: if you retry a create
after the idempotency record expires, the unique constraint fires and you get
**409 `external_reference_taken`** instead of a duplicate link. Treat that code
as "already done, go look it up" rather than as an error.

The reference is one URL path segment, so percent-encode it if it can contain
`/` or `#`. A `:` is fine unencoded. Conventional shapes:

```text
document:0198c7e2-4f1a-7c3d-9b21-5f0a2c8e1d44
invoice:12345
payslip:2026-08:user-193
```

The shortener never needs to understand the document. It stores and redirects to
the destination URL you supply, and nothing else.

## PHP / Laravel

Register the config, then drop in the client.

```php
// config/services.php
'shorturl' => [
    'base_url' => env('SHORTURL_BASE_URL', 'https://short.example.com/api/v1'),
    'key'      => env('SHORTURL_API_KEY'),
],
```

```php
<?php
// app/Services/ShortUrl/ShortUrlException.php

namespace App\Services\ShortUrl;

use RuntimeException;

class ShortUrlException extends RuntimeException
{
    public function __construct(
        public readonly string $errorCode,
        string $message,
        public readonly int $status = 0,
        public readonly ?string $requestId = null,
        public readonly array $fields = [],
    ) {
        parent::__construct($message);
    }
}
```

```php
<?php
// app/Services/ShortUrl/ShortUrlClient.php

namespace App\Services\ShortUrl;

use Illuminate\Http\Client\PendingRequest;
use Illuminate\Http\Client\Response;
use Illuminate\Support\Facades\Http;

class ShortUrlClient
{
    public function __construct(
        private readonly string $baseUrl,
        private readonly string $apiKey,
    ) {
    }

    /**
     * Shorten a URL for a caller-owned entity, safely retryable.
     *
     * @param  array<string, mixed>  $extra  Any other CreateLinkRequest fields.
     * @return array<string, mixed> The link resource (data.*).
     */
    public function shortenFor(string $reference, string $destinationUrl, array $extra = []): array
    {
        $response = $this->request()
            ->withHeaders(['Idempotency-Key' => $reference])
            ->post('/links', array_merge($extra, [
                'destination_url'    => $destinationUrl,
                'external_reference' => $reference,
            ]));

        // The idempotency window is 24h; past it, a retry hits the unique
        // constraint instead. Same outcome the caller wanted: the existing link.
        if ($response->status() === 409
            && $response->json('error.code') === 'external_reference_taken') {
            return $this->findByReference($reference)
                ?? throw $this->toException($response);
        }

        return $this->unwrap($response);
    }

    /** @return array<string, mixed>|null Null when nothing was ever shortened for this reference. */
    public function findByReference(string $reference): ?array
    {
        $response = $this->request()->get('/links/by-reference/'.rawurlencode($reference));

        if ($response->status() === 404) {
            return null;
        }

        return $this->unwrap($response);
    }

    /** @return array<string, mixed> */
    public function get(string $linkId): array
    {
        return $this->unwrap($this->request()->get("/links/{$linkId}"));
    }

    /**
     * @param  array<string, mixed>  $changes  Partial update; explicit nulls clear a field.
     * @return array<string, mixed>
     */
    public function update(string $linkId, array $changes): array
    {
        return $this->unwrap($this->request()->patch("/links/{$linkId}", $changes));
    }

    /** Soft-disable. The link stops redirecting; history and analytics survive. */
    public function disable(string $linkId): void
    {
        $this->unwrap($this->request()->delete("/links/{$linkId}"));
    }

    /**
     * Irreversible. Deletes the row and its analytics, and frees the slug.
     *
     * `hard=true` must be in the query string: with asJson() set, passing it as
     * $data would send it as a JSON body, which the server never reads — and
     * you would silently get a soft disable instead.
     */
    public function deleteForever(string $linkId): void
    {
        $this->unwrap($this->request()->delete("/links/{$linkId}?hard=true"));
    }

    private function request(): PendingRequest
    {
        return Http::baseUrl($this->baseUrl)
            ->withToken($this->apiKey)
            ->acceptJson()
            ->asJson()
            ->timeout(10)
            ->connectTimeout(5)
            // Retry only what is worth retrying. 4xx other than 429 will never
            // succeed on a second attempt; throw: false lets us read the body.
            ->retry(3, 250, function ($exception, $request) {
                $response = $exception->response ?? null;

                return $response === null
                    || $response->status() === 429
                    || $response->serverError();
            }, throw: false);
    }

    /** @return array<string, mixed> */
    private function unwrap(Response $response): array
    {
        if ($response->failed()) {
            throw $this->toException($response);
        }

        return $response->status() === 204 ? [] : ($response->json('data') ?? []);
    }

    private function toException(Response $response): ShortUrlException
    {
        return new ShortUrlException(
            errorCode: $response->json('error.code') ?? 'unknown',
            message: $response->json('error.message') ?? 'ShortURL request failed',
            status: $response->status(),
            requestId: $response->json('error.request_id'),
            fields: $response->json('error.fields') ?? [],
        );
    }
}
```

```php
<?php
// app/Providers/AppServiceProvider.php — in register()

use App\Services\ShortUrl\ShortUrlClient;

$this->app->singleton(ShortUrlClient::class, fn () => new ShortUrlClient(
    baseUrl: config('services.shorturl.base_url'),
    apiKey:  config('services.shorturl.key'),
));
```

Using it:

```php
<?php
// app/Http/Controllers/DocumentShareController.php

namespace App\Http\Controllers;

use App\Models\Document;
use App\Services\ShortUrl\ShortUrlClient;
use Illuminate\Support\Facades\URL;

class DocumentShareController extends Controller
{
    public function share(Document $document, ShortUrlClient $shortUrl): string
    {
        try {
            $link = $shortUrl->shortenFor(
                reference: "document:{$document->id}",
                destinationUrl: $this->buildSecureDocumentUrl($document),
                extra: [
                    'title'    => $document->title,
                    'metadata' => ['source' => 'intranet', 'document_type' => $document->type],
                ],
            );

            return $link['short_url'];
        } catch (\Throwable $e) {
            // ShortUrlException for an API-level failure; ConnectionException
            // still escapes retry() when every attempt times out, so catch
            // broadly here.
            report($e);

            // Sharing the long URL beats not sharing at all.
            return $this->buildSecureDocumentUrl($document);
        }
    }

    private function buildSecureDocumentUrl(Document $document): string
    {
        return URL::temporarySignedRoute(
            'documents.show', now()->addDays(7), ['document' => $document->id],
        );
    }
}
```

## Python

```python
"""Minimal ShortURL client. Requires `requests`."""

from __future__ import annotations

import os
from typing import Any
from urllib.parse import quote

import requests
from requests.adapters import HTTPAdapter
from urllib3.util.retry import Retry


class ShortUrlError(RuntimeError):
    def __init__(self, code: str, message: str, status: int, request_id: str | None = None,
                 fields: dict[str, list[str]] | None = None) -> None:
        super().__init__(f"{code}: {message}")
        self.code = code
        self.status = status
        self.request_id = request_id
        self.fields = fields or {}


class ShortUrlClient:
    def __init__(self, base_url: str, api_key: str, timeout: float = 10.0) -> None:
        self.base_url = base_url.rstrip("/")
        self.timeout = timeout

        self.session = requests.Session()
        self.session.headers.update({
            "Authorization": f"Bearer {api_key}",
            "Content-Type": "application/json",
            "Accept": "application/json",
        })
        # Retry only 429 and 5xx. POST is in allowed_methods because every POST
        # this client makes carries an Idempotency-Key, so a retry cannot
        # duplicate a link. Drop POST if you ever call the API without one.
        #
        # respect_retry_after_header honours the Retry-After: 60 the API sends
        # with a 429 — which means a rate-limited call can block for a minute.
        # Set respect_retry_after_header=False, or run this off a queue, if you
        # are on a user-facing request path.
        retry = Retry(
            total=3,
            backoff_factor=0.5,
            status_forcelist=(429, 500, 502, 503, 504),
            allowed_methods=frozenset({"GET", "POST", "PATCH", "DELETE"}),
            respect_retry_after_header=True,
            raise_on_status=False,
        )
        self.session.mount("https://", HTTPAdapter(max_retries=retry))
        self.session.mount("http://", HTTPAdapter(max_retries=retry))

    def shorten_for(self, reference: str, destination_url: str, **extra: Any) -> dict[str, Any]:
        """Shorten a URL for a caller-owned reference. Safe to call twice."""
        response = self.session.post(
            f"{self.base_url}/links",
            headers={"Idempotency-Key": reference},
            json={"destination_url": destination_url, "external_reference": reference, **extra},
            timeout=self.timeout,
        )

        # Past the 24h idempotency window the unique constraint answers instead.
        if response.status_code == 409 and self._code(response) == "external_reference_taken":
            existing = self.find_by_reference(reference)
            if existing is not None:
                return existing

        return self._unwrap(response)

    def find_by_reference(self, reference: str) -> dict[str, Any] | None:
        response = self.session.get(
            f"{self.base_url}/links/by-reference/{quote(reference, safe='')}",
            timeout=self.timeout,
        )
        if response.status_code == 404:
            return None
        return self._unwrap(response)

    def get(self, link_id: str) -> dict[str, Any]:
        return self._unwrap(
            self.session.get(f"{self.base_url}/links/{link_id}", timeout=self.timeout)
        )

    def update(self, link_id: str, **changes: Any) -> dict[str, Any]:
        return self._unwrap(
            self.session.patch(f"{self.base_url}/links/{link_id}", json=changes,
                               timeout=self.timeout)
        )

    def list_page(self, **filters: Any) -> tuple[list[dict[str, Any]], str | None]:
        """One page. Pass cursor=<next_cursor> for the following page."""
        params = {k: v for k, v in filters.items() if v is not None}
        response = self.session.get(f"{self.base_url}/links", params=params,
                                    timeout=self.timeout)
        body = self._json(response)
        if not response.ok:
            raise self._error(response, body)
        return body["data"], body["meta"]["next_cursor"]

    def iter_all(self, **filters: Any):
        """Walk every page. Filters other than `cursor` are passed through."""
        filters.pop("cursor", None)
        cursor: str | None = None
        while True:
            page, cursor = self.list_page(cursor=cursor, **filters)
            yield from page
            if cursor is None:
                return

    def disable(self, link_id: str) -> None:
        """Soft-disable: stops redirecting, keeps history."""
        self._unwrap(self.session.delete(f"{self.base_url}/links/{link_id}",
                                         timeout=self.timeout))

    def delete_forever(self, link_id: str) -> None:
        """Irreversible: removes the row and its analytics."""
        self._unwrap(self.session.delete(f"{self.base_url}/links/{link_id}",
                                         params={"hard": "true"}, timeout=self.timeout))

    # --- internals ---

    @staticmethod
    def _json(response: requests.Response) -> dict[str, Any]:
        try:
            return response.json()
        except ValueError:
            return {}

    @classmethod
    def _code(cls, response: requests.Response) -> str:
        return cls._json(response).get("error", {}).get("code", "unknown")

    def _unwrap(self, response: requests.Response) -> dict[str, Any]:
        if response.status_code == 204:
            return {}
        body = self._json(response)
        if not response.ok:
            raise self._error(response, body)
        return body.get("data", {})

    @staticmethod
    def _error(response: requests.Response, body: dict[str, Any]) -> ShortUrlError:
        err = body.get("error", {})
        return ShortUrlError(
            code=err.get("code", "unknown"),
            message=err.get("message", response.text or "request failed"),
            status=response.status_code,
            request_id=err.get("request_id"),
            fields=err.get("fields"),
        )


if __name__ == "__main__":
    client = ShortUrlClient(
        base_url=os.environ.get("SHORTURL_BASE_URL", "https://short.example.com/api/v1"),
        api_key=os.environ["SHORTURL_API_KEY"],
    )

    document_id = "0198c7e2-4f1a-7c3d-9b21-5f0a2c8e1d44"
    link = client.shorten_for(
        reference=f"document:{document_id}",
        destination_url=f"https://docs.internal.example.com/documents/{document_id}",
        title="Q3 Handover Pack",
        metadata={"source": "intranet"},
    )
    print(link["short_url"])
```

## Node.js

No dependencies; `fetch` is built in from Node 18.

```js
// shorturl.mjs
export class ShortUrlError extends Error {
  constructor({ code, message, status, requestId, fields }) {
    super(`${code}: ${message}`);
    this.name = "ShortUrlError";
    this.code = code;
    this.status = status;
    this.requestId = requestId;
    this.fields = fields ?? {};
  }
}

const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

export class ShortUrlClient {
  /** @param {{baseUrl?: string, apiKey: string, timeoutMs?: number, maxRetries?: number}} options */
  constructor({
    baseUrl = "https://short.example.com/api/v1",
    apiKey,
    timeoutMs = 10_000,
    maxRetries = 3,
  }) {
    if (!apiKey) throw new Error("apiKey is required");
    this.baseUrl = baseUrl.replace(/\/$/, "");
    this.apiKey = apiKey;
    this.timeoutMs = timeoutMs;
    this.maxRetries = maxRetries;
  }

  /** Shorten a URL for a caller-owned reference. Safe to call twice. */
  async shortenFor(reference, destinationUrl, extra = {}) {
    const response = await this.#send("POST", "/links", {
      headers: { "Idempotency-Key": reference },
      body: { destination_url: destinationUrl, external_reference: reference, ...extra },
    });

    // Past the 24h idempotency window the unique constraint answers instead.
    if (response.status === 409) {
      const body = await readJson(response);
      if (body?.error?.code === "external_reference_taken") {
        const existing = await this.findByReference(reference);
        if (existing) return existing;
      }
      throw toError(response, body);
    }

    return unwrap(response);
  }

  /** @returns {Promise<object|null>} null when nothing was ever shortened for this reference. */
  async findByReference(reference) {
    const path = `/links/by-reference/${encodeURIComponent(reference)}`;
    const response = await this.#send("GET", path);
    if (response.status === 404) return null;
    return unwrap(response);
  }

  async get(linkId) {
    return unwrap(await this.#send("GET", `/links/${linkId}`));
  }

  /** Partial update. Explicit nulls clear title, expires_at, max_clicks, password, metadata. */
  async update(linkId, changes) {
    return unwrap(await this.#send("PATCH", `/links/${linkId}`, { body: changes }));
  }

  /** One page. Returns { links, nextCursor }. */
  async list(filters = {}) {
    const query = new URLSearchParams(
      Object.entries(filters).filter(([, v]) => v !== undefined && v !== null),
    );
    const response = await this.#send("GET", `/links?${query}`);
    const body = await readJson(response);
    if (!response.ok) throw toError(response, body);
    return { links: body.data, nextCursor: body.meta.next_cursor };
  }

  async *iterAll(filters = {}) {
    let cursor;
    do {
      const { links, nextCursor } = await this.list({ ...filters, cursor });
      yield* links;
      cursor = nextCursor ?? undefined;
    } while (cursor);
  }

  /** Soft-disable: stops redirecting, keeps history. */
  async disable(linkId) {
    await unwrap(await this.#send("DELETE", `/links/${linkId}`));
  }

  /** Irreversible: removes the row and its analytics. */
  async deleteForever(linkId) {
    await unwrap(await this.#send("DELETE", `/links/${linkId}?hard=true`));
  }

  async #send(method, path, { headers = {}, body } = {}) {
    const backoffMs = (attempt) => 250 * 2 ** attempt;

    for (let attempt = 0; ; attempt++) {
      let response;
      try {
        response = await fetch(`${this.baseUrl}${path}`, {
          method,
          headers: {
            Authorization: `Bearer ${this.apiKey}`,
            Accept: "application/json",
            ...(body !== undefined ? { "Content-Type": "application/json" } : {}),
            ...headers,
          },
          body: body !== undefined ? JSON.stringify(body) : undefined,
          signal: AbortSignal.timeout(this.timeoutMs),
        });
      } catch (error) {
        // Network failure or timeout. Retrying a POST is safe here only
        // because shortenFor always sends an Idempotency-Key.
        if (attempt >= this.maxRetries) throw error;
        await sleep(backoffMs(attempt));
        continue;
      }

      // Only 429 and 5xx can succeed on a retry. Everything else is final.
      if (response.status !== 429 && response.status < 500) return response;
      if (attempt >= this.maxRetries) return response;

      // Retry-After is 60 on this API. On a user-facing request path you
      // probably want to give up now and queue the work instead.
      const retryAfter = Number(response.headers.get("Retry-After"));
      await sleep(
        Number.isFinite(retryAfter) && retryAfter > 0
          ? retryAfter * 1000
          : backoffMs(attempt),
      );
    }
  }
}

async function readJson(response) {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

function toError(response, body) {
  const error = body?.error ?? {};
  return new ShortUrlError({
    code: error.code ?? "unknown",
    message: error.message ?? response.statusText,
    status: response.status,
    requestId: error.request_id,
    fields: error.fields,
  });
}

async function unwrap(response) {
  if (response.status === 204) return null;
  const body = await readJson(response);
  if (!response.ok) throw toError(response, body);
  return body.data;
}
```

```js
// usage.mjs
import { ShortUrlClient, ShortUrlError } from "./shorturl.mjs";

const client = new ShortUrlClient({
  baseUrl: process.env.SHORTURL_BASE_URL ?? "https://short.example.com/api/v1",
  apiKey: process.env.SHORTURL_API_KEY,
});

const documentId = "0198c7e2-4f1a-7c3d-9b21-5f0a2c8e1d44";

try {
  const link = await client.shortenFor(
    `document:${documentId}`,
    `https://docs.internal.example.com/documents/${documentId}`,
    { title: "Q3 Handover Pack", metadata: { source: "intranet" } },
  );
  console.log(link.short_url);
} catch (error) {
  if (error instanceof ShortUrlError) {
    console.error(`ShortURL ${error.code} (request ${error.requestId})`);
  }
  throw error;
}
```

## Error handling

Branch on `error.code`, never on `error.message` — the message is written for
humans and can change between releases. Quote `error.request_id` when you report
a problem to whoever runs the installation; it matches their log line.

```json
{
  "error": {
    "code": "validation_error",
    "message": "The request is invalid",
    "fields": { "destination_url": ["must include a scheme, for example https://"] },
    "request_id": "3f9a1c2e"
  }
}
```

| Code | HTTP | Cause | What to do |
|---|---|---|---|
| `bad_request` | 400 | Body is not valid JSON, is empty, is over 1 MB, `Content-Type` is not `application/json`, an `Idempotency-Key` over 255 chars, an unparseable `cursor`, a `created_after`/`created_before` that is not RFC 3339, or a `domain_id` that is not a UUID. | Fix the caller. Never retry unchanged. |
| `unauthorized` | 401 | Key missing, malformed, unknown, revoked, or expired — all five look identical. | Check the `Authorization: Bearer` header, then check the key is still live in the dashboard. Do not retry. |
| `forbidden` | 403 | Generic authorization failure. | Rare on this API. Treat as `insufficient_scope`. |
| `insufficient_scope` | 403 | The key authenticated but was issued without the scope this route needs. The message names the missing scope. | Issue a new key with the right scopes. Scopes cannot be widened after creation. Do not retry. |
| `not_found` | 404 | No such link in *this key's* workspace. A link in another workspace and a `linkId` that is not a valid UUID both land here. | For `by-reference` lookups this is the normal "not shortened yet" answer — handle it, do not log it as an error. Otherwise check you are using the right key. |
| `slug_taken` | 409 | You asked for a specific `slug` that already exists on that domain. | Pick another slug, or drop `slug` and let the server generate one. Retrying identically will always fail. |
| `external_reference_taken` | 409 | A link with that `external_reference` already exists in the workspace. | You already shortened this. `GET /links/by-reference/{ref}` and use what comes back. Usually not an error at all. |
| `no_default_domain` | 409 | You sent no `domain`/`domain_id` and the workspace has no verified default domain. | An operator must connect and verify a domain in the dashboard. Nothing the caller can change fixes this — surface it to a human. |
| `domain_not_active` | 409 | The domain you named exists but is not `active` (still pending DNS or certificate issuance). The message names the actual status. | Wait for verification to finish, or target a different domain. Retry later, not immediately. |
| `idempotency_conflict` | 409 | The same `Idempotency-Key` was used before with a *different* body, within 24 hours. | **This is a bug in the caller.** You reused a key for different content — usually a key built from something not unique enough, or a body that varies between attempts (a timestamp, a freshly minted token in `destination_url`, a `metadata` field carrying a retry count). Do not retry: fix the key derivation or stop varying the body. |
| `idempotency_link_gone` | 409 | The key is known and the body matches, but the link it created has since been deleted. | Retry once with a fresh key. |
| `conflict` | 409 | Generic state conflict. | Read the message. Not emitted by the link endpoints today. |
| `validation_error` | 422 | A field failed validation, or the body carried an unknown field (reported as `"unknown field"`). `error.fields` lists every offender. | Fix the caller and surface `fields` in your logs. Never retry unchanged. |
| `rate_limited` | 429 | The key exceeded its per-minute allowance. Nothing was created or changed. | Wait `Retry-After` seconds, then retry. See below. |
| `internal_error` | 500 | Something failed server-side. The body carries no detail beyond `request_id` by design. | Retry with backoff. If you sent an `Idempotency-Key`, the retry is safe even if the first attempt actually succeeded. Report `request_id` if it persists. |

Two rules that save the most time:

- **Retry only 429, 5xx, and network failures.** A 4xx other than 429 will never
  succeed on a second identical attempt; retrying it just burns your rate limit.
- **Always send an `Idempotency-Key` on `POST /links`.** It is the difference
  between a safe retry and a pile of duplicate short URLs.

## Rate limits

**120 requests per minute per API key**, by default. The operator changes it with
`RATE_LIMIT_API_PER_MINUTE` in `.env`; ask them what yours is rather than
hardcoding 120.

The limit is keyed on the API key, not the client IP. Several application
servers sharing one key share one allowance, and two teams behind the same NAT
do not interfere with each other. If one integration needs its own headroom,
give it its own key.

It is a token bucket with a burst equal to the full allowance: 120 requests can
land at once, then the bucket refills at 2 per second. A short spike is fine; a
sustained 3-per-second stream is not.

What the server sends:

| Header | When |
|---|---|
| `X-RateLimit-Limit` | On every response that passed authentication. |
| `Retry-After: 60` | On every 429. |

There is no `X-RateLimit-Remaining` and no `X-RateLimit-Reset`, so you cannot
see yourself approaching the wall — you find out by getting a 429. Plan for it
rather than trying to predict it:

- Honour `Retry-After` when it is present; fall back to exponential backoff with
  jitter otherwise. All three clients above do this.
- Cap retries — three attempts is plenty. A 429 means the limiter is already
  saturated; hammering it keeps it saturated.
- Do bulk work through a queue with limited concurrency rather than a `for` loop
  firing parallel requests. Shortening 10,000 documents at 2 requests per second
  takes about 85 minutes and never trips the limiter.
- Do not poll `GET /links` to check whether something exists. Use
  `GET /links/by-reference/{ref}`, or better, remember the ID.

The limiter is in-memory and per-container. In the default single-container
deployment that is exactly one bucket. If the stack is ever scaled to several
server containers, each enforces its own share — see `docs/deployment.md`.

## Security: a short URL is not an authorization boundary

**A short URL is an identifier and a redirect. It is not a permission check.**
Anyone who has the short URL can follow it, and it is short enough to guess at
far more cheaply than a signed 200-character document URL. Short URLs travel:
they get forwarded, pasted into group chats, screenshotted, indexed by whatever
scans the recipient's inbox, and logged by every proxy in between.

**The destination system must keep enforcing its own authentication and
authorization on every request.** If `https://docs.internal.example.com/documents/0198c7e2?token=…`
hands the document to anyone who requests it, then shortening it has published
that document to anyone who ends up with the short link. That was true of the
long URL too — the shortener just makes it far easier to pass around.

Concretely:

- Keep the session check, the signed-URL expiry, and the per-user access check
  on the destination. The shortener never sees the document and cannot make any
  judgement about who may read it.
- `expires_at` and `max_clicks` are convenience, not enforcement. They stop the
  *redirect*; anyone who already resolved the short URL still has the long one
  and can go straight there. Expire the underlying token as well.
- The `password` field is a speed bump for casual sharing, not access control.
  It gates the redirect page, not the destination.
- Do not put secrets in `slug`, `title`, or `metadata`. Slugs are visible to
  every visitor; the rest is readable by anyone with a `links:read` key or
  dashboard access to the workspace.
- Prefer short `expires_at` values for links to sensitive documents, and disable
  the link (`DELETE /links/{id}`, the soft form) when the share is withdrawn —
  then revoke the underlying token too.
- Treat the API key like a password. It grants workspace-wide read and write for
  as long as it exists, outliving any user session. Store it in a secret
  manager, never in source control, and revoke it the moment it may have leaked.
