<script setup lang="ts">
import type { ApiKey, CreatedApiKey } from '~/types/api'
import { formatDateTime } from '~/components/links/format'
import { toRfc3339 } from '~/components/links/form'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'API keys · ShortURL' })

const ws = useWorkspaces()
const toast = useToast()
const config = useRuntimeConfig()
const { apiKeys } = useServices()

const canManage = computed(() => ws.role.value === 'owner' || ws.role.value === 'admin')

// -------------------------------------------------------------------- list

const keys = ref<ApiKey[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)

async function load() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || !canManage.value) {
    loading.value = false
    return
  }
  loading.value = true
  loadError.value = null
  try {
    const res = await apiKeys.list(workspaceId)
    keys.value = res.data
  } catch (error) {
    loadError.value = error instanceof ApiError ? error.message : 'Could not load API keys.'
  } finally {
    loading.value = false
  }
}

watch([() => ws.activeId.value, canManage], () => load(), { immediate: true })

function keyState(key: ApiKey): { label: string, tone: 'success' | 'neutral' | 'warning' } {
  if (key.revoked_at) return { label: 'Revoked', tone: 'neutral' }
  if (key.expires_at && new Date(key.expires_at).getTime() < Date.now()) {
    return { label: 'Expired', tone: 'warning' }
  }
  return { label: 'Active', tone: 'success' }
}

// ------------------------------------------------------------------ create

const ALL_SCOPES = [
  { value: 'links:read', label: 'links:read', hint: 'Read links and look them up by reference.' },
  { value: 'links:write', label: 'links:write', hint: 'Create, update and delete links.' },
  { value: 'analytics:read', label: 'analytics:read', hint: 'Read click reports.' },
]

const createOpen = ref(false)
const creating = ref(false)
const newName = ref('')
const newScopes = ref<string[]>(['links:read', 'links:write'])
const newExpiresAt = ref('')
const newTest = ref(false)
const createError = ref<string | null>(null)

function openCreate() {
  newName.value = ''
  newScopes.value = ['links:read', 'links:write']
  newExpiresAt.value = ''
  newTest.value = false
  createError.value = null
  createOpen.value = true
}

/** The plaintext key, shown exactly once. Nothing dismisses this but the user. */
const revealed = ref<CreatedApiKey | null>(null)

async function createKey() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || creating.value) return

  const name = newName.value.trim()
  if (!name) {
    createError.value = 'Give the key a name so you can tell it apart later.'
    return
  }
  if (!newScopes.value.length) {
    createError.value = 'Pick at least one scope.'
    return
  }

  const expiresAt = toRfc3339(newExpiresAt.value)
  if (newExpiresAt.value && (!expiresAt || new Date(expiresAt).getTime() <= Date.now())) {
    createError.value = 'Expiration time must be in the future.'
    return
  }

  creating.value = true
  createError.value = null
  try {
    const body: { name: string, scopes: string[], expires_at?: string, test: boolean } = {
      name,
      scopes: [...newScopes.value],
      test: newTest.value,
    }
    if (expiresAt) body.expires_at = expiresAt

    const created = await apiKeys.create(workspaceId, body)
    keys.value = [created, ...keys.value]
    createOpen.value = false
    revealed.value = created
  } catch (error) {
    if (error instanceof ApiError) {
      createError.value = error.field('name') ?? error.field('scopes') ?? error.field('expires_at') ?? error.message
    } else {
      createError.value = 'Could not create the key.'
    }
  } finally {
    creating.value = false
  }
}

// ------------------------------------------------------------------ revoke

const revokeTarget = ref<ApiKey | null>(null)
const revokeOpen = ref(false)
const revoking = ref(false)

function askRevoke(key: ApiKey) {
  revokeTarget.value = key
  revokeOpen.value = true
}

async function confirmRevoke() {
  const workspaceId = ws.activeId.value
  const target = revokeTarget.value
  if (!workspaceId || !target) return

  revoking.value = true
  try {
    await apiKeys.revoke(workspaceId, target.id)
    await load()
    toast.success(`Revoked ${target.name}`)
    revokeOpen.value = false
    revokeTarget.value = null
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not revoke the key.')
  } finally {
    revoking.value = false
  }
}

// ------------------------------------------------------------------ usage

const origin = ref('')
onMounted(() => {
  origin.value = window.location.origin
})

const apiBase = computed(() => {
  const base = String(config.public.apiBaseUrl || '/api/v1')
  if (/^https?:\/\//.test(base)) return base
  return `${origin.value || 'https://your-shorturl-host'}${base}`
})

const snippet = computed(() => `curl -X POST ${apiBase.value}/links \\
  -H "Authorization: Bearer shr_live_..." \\
  -H "Content-Type: application/json" \\
  -H "Idempotency-Key: invoice-12345" \\
  -d '{"destination_url":"https://example.com/very/long/url"}'`)
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-xl font-semibold tracking-tight">
          API keys
        </h1>
        <p class="mt-0.5 text-sm text-(--color-content-muted)">
          Machine-to-machine access for your own systems. A key is bound to this workspace.
        </p>
      </div>
      <UiButton v-if="canManage" @click="openCreate">
        Create key
      </UiButton>
    </header>

    <UiCard v-if="!canManage" :padded="false">
      <UiEmptyState
        title="You do not have access"
        description="API keys grant workspace-wide access that outlives any session, so only owners and admins can see or issue them. Ask an admin if you need one."
      >
        <UiButton variant="secondary" to="/dashboard">
          Back to overview
        </UiButton>
      </UiEmptyState>
    </UiCard>

    <template v-else>
      <!-- Shown once, and only the user closes it. Plan §38. -->
      <section
        v-if="revealed"
        class="rounded-lg border-2 border-(--color-warning) bg-amber-500/10 p-5"
        role="alert"
        aria-labelledby="revealed-heading"
      >
        <h2 id="revealed-heading" class="font-semibold text-(--color-content)">
          Copy your API key now
        </h2>
        <p class="mt-1 text-sm text-(--color-content)">
          This is the only time <strong>{{ revealed.name }}</strong> will ever be shown.
          It is stored as a hash — nobody, including this server, can display it again.
          If you lose it you must revoke the key and create a new one.
        </p>

        <div class="mt-4">
          <UiCopyButton :value="revealed.key" show-value label="Copy key" />
        </div>

        <div class="mt-4">
          <UiButton variant="secondary" @click="revealed = null">
            I've saved it
          </UiButton>
        </div>
      </section>

      <!-- Keys -->
      <UiCard :padded="false">
        <p v-if="loading" class="px-5 py-10 text-center text-sm text-(--color-content-muted)" role="status">
          Loading keys…
        </p>

        <div v-else-if="loadError" class="px-5 py-10 text-center">
          <p class="text-sm text-(--color-danger)" role="alert">
            {{ loadError }}
          </p>
          <UiButton class="mt-3" variant="secondary" size="sm" @click="load">
            Try again
          </UiButton>
        </div>

        <UiEmptyState
          v-else-if="!keys.length"
          title="No API keys yet"
          description="Create a key so an internal system can request short links without a browser session."
        >
          <UiButton @click="openCreate">
            Create your first key
          </UiButton>
        </UiEmptyState>

        <div v-else class="overflow-x-auto">
          <table class="w-full min-w-[56rem] text-left text-sm">
            <thead>
              <tr class="border-b border-(--color-border) text-xs uppercase tracking-wide text-(--color-content-subtle)">
                <th scope="col" class="px-5 py-3 font-medium">
                  Name
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Prefix
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Scopes
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Last used
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Created
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Expires
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  Status
                </th>
                <th scope="col" class="px-5 py-3 font-medium">
                  <span class="sr-only">Actions</span>
                </th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="key in keys"
                :key="key.id"
                class="border-b border-(--color-border) last:border-0"
              >
                <td class="px-5 py-3 font-medium">
                  {{ key.name }}
                </td>
                <td class="px-5 py-3">
                  <code class="font-mono text-xs text-(--color-content-muted)">{{ key.key_prefix }}…</code>
                </td>
                <td class="px-5 py-3">
                  <div class="flex flex-wrap gap-1">
                    <UiBadge v-for="scope in key.scopes" :key="scope" tone="info">
                      {{ scope }}
                    </UiBadge>
                    <span v-if="!key.scopes.length" class="text-xs text-(--color-content-subtle)">none</span>
                  </div>
                </td>
                <td class="whitespace-nowrap px-5 py-3 text-(--color-content-muted)">
                  {{ formatDateTime(key.last_used_at) }}
                </td>
                <td class="whitespace-nowrap px-5 py-3 text-(--color-content-muted)">
                  {{ formatDateTime(key.created_at) }}
                </td>
                <td class="whitespace-nowrap px-5 py-3 text-(--color-content-muted)">
                  {{ key.expires_at ? formatDateTime(key.expires_at) : 'Never' }}
                </td>
                <td class="px-5 py-3">
                  <UiBadge :tone="keyState(key).tone" dot>
                    {{ keyState(key).label }}
                  </UiBadge>
                </td>
                <td class="px-5 py-3 text-right">
                  <UiButton
                    v-if="!key.revoked_at"
                    variant="ghost"
                    size="sm"
                    @click="askRevoke(key)"
                  >
                    <span class="text-(--color-danger)">Revoke</span>
                  </UiButton>
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </UiCard>

      <!-- Usage -->
      <UiCard title="Using a key" description="Send the key as a bearer token. The workspace comes from the key, never from the request.">
        <template #actions>
          <UiCopyButton :value="snippet" label="Copy snippet" />
        </template>
        <pre class="overflow-x-auto rounded-md bg-(--color-surface-muted) p-4 font-mono text-xs leading-relaxed"><code>{{ snippet }}</code></pre>
        <p class="mt-3 text-sm text-(--color-content-muted)">
          <code class="font-mono text-xs">Idempotency-Key</code> is optional but recommended: retrying a
          request with the same key returns the original link instead of creating a duplicate.
        </p>
      </UiCard>
    </template>

    <!-- Create -->
    <UiModal v-model="createOpen" title="Create an API key" description="Scopes cannot be changed later — create a new key instead.">
      <form class="flex flex-col gap-4" @submit.prevent="createKey">
        <UiInput
          v-model="newName"
          label="Name"
          required
          placeholder="ERP Production"
          :disabled="creating"
        />

        <fieldset class="flex flex-col gap-2">
          <legend class="text-sm font-medium">
            Scopes
          </legend>
          <label v-for="scope in ALL_SCOPES" :key="scope.value" class="flex items-start gap-2 text-sm">
            <input
              v-model="newScopes"
              type="checkbox"
              :value="scope.value"
              :disabled="creating"
              class="mt-0.5 size-4 rounded border-(--color-border-strong)"
            >
            <span>
              <code class="font-mono text-xs">{{ scope.label }}</code>
              <span class="block text-xs text-(--color-content-muted)">{{ scope.hint }}</span>
            </span>
          </label>
        </fieldset>

        <UiInput
          v-model="newExpiresAt"
          label="Expiration time (optional)"
          type="datetime-local"
          :disabled="creating"
          hint="Leave empty for a key that never expires. Time is interpreted in your local timezone."
        />

        <label class="flex items-start gap-2 text-sm">
          <input
            v-model="newTest"
            type="checkbox"
            :disabled="creating"
            class="mt-0.5 size-4 rounded border-(--color-border-strong)"
          >
          <span>
            Test key
            <span class="block text-xs text-(--color-content-muted)">
              Issues a <code class="font-mono">shr_test_</code> prefix instead of
              <code class="font-mono">shr_live_</code>. Behaviour is identical; the prefix
              just makes the environment obvious in your own logs.
            </span>
          </span>
        </label>

        <p v-if="createError" class="text-sm text-(--color-danger)" role="alert">
          {{ createError }}
        </p>
      </form>

      <template #actions>
        <UiButton variant="secondary" :disabled="creating" @click="createOpen = false">
          Cancel
        </UiButton>
        <UiButton :loading="creating" @click="createKey">
          Create key
        </UiButton>
      </template>
    </UiModal>

    <!-- Revoke -->
    <UiModal
      v-model="revokeOpen"
      title="Revoke this key?"
      description="Any integration still using it starts failing immediately with a 401. This cannot be undone."
      danger
    >
      <p v-if="revokeTarget" class="rounded-md bg-(--color-surface-muted) px-3 py-2 text-sm">
        <span class="font-medium">{{ revokeTarget.name }}</span>
        <code class="ml-2 font-mono text-xs text-(--color-content-muted)">{{ revokeTarget.key_prefix }}…</code>
      </p>

      <template #actions>
        <UiButton variant="secondary" @click="revokeOpen = false">
          Cancel
        </UiButton>
        <UiButton variant="danger" :loading="revoking" @click="confirmRevoke">
          Revoke key
        </UiButton>
      </template>
    </UiModal>
  </div>
</template>
