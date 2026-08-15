<script setup lang="ts">
import type { DnsInstructions, Domain } from '~/types/api'
import DomainDnsRecords from '~/components/domains/DnsRecords.vue'
import DomainStatusBadge from '~/components/domains/StatusBadge.vue'
import { formatDateTime } from '~/components/links/format'
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ middleware: 'auth' })

useHead({ title: 'Domains · ShortURL' })

const ws = useWorkspaces()
const toast = useToast()
const { domains } = useServices()

const canManage = computed(() => ws.role.value === 'owner' || ws.role.value === 'admin')

// ------------------------------------------------------------------ list

const list = ref<Domain[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)

/** Keyed by domain ID. The list endpoint omits DNS instructions on purpose —
 *  it would hand out every verification token on a page people only skim — so
 *  they are fetched per domain when a row is expanded. */
const instructions = ref<Record<string, DnsInstructions>>({})
const expanded = ref<Record<string, boolean>>({})
const fetchingInstructions = ref<Record<string, boolean>>({})
const verifying = ref<Record<string, boolean>>({})
const settingDefault = ref<Record<string, boolean>>({})

async function load() {
  const workspaceId = ws.activeId.value
  if (!workspaceId) {
    loading.value = false
    return
  }
  loading.value = true
  loadError.value = null
  try {
    const res = await domains.list(workspaceId)
    list.value = res.data
  } catch (error) {
    loadError.value = error instanceof ApiError ? error.message : 'Could not load domains.'
  } finally {
    loading.value = false
  }
}

watch(() => ws.activeId.value, () => {
  expanded.value = {}
  instructions.value = {}
  load()
}, { immediate: true })

function replaceDomain(next: Domain) {
  list.value = list.value.map(d => (d.id === next.id ? next : d))
  if (next.dns_instructions) instructions.value[next.id] = next.dns_instructions
}

async function fetchInstructions(domain: Domain) {
  const workspaceId = ws.activeId.value
  if (!workspaceId || instructions.value[domain.id]) return

  fetchingInstructions.value[domain.id] = true
  try {
    const full = await domains.get(workspaceId, domain.id)
    if (full.dns_instructions) instructions.value[domain.id] = full.dns_instructions
    replaceDomain(full)
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not load the DNS records.')
  } finally {
    fetchingInstructions.value[domain.id] = false
  }
}

async function toggleRow(domain: Domain) {
  const isOpen = expanded.value[domain.id]
  expanded.value[domain.id] = !isOpen
  if (!isOpen) await fetchInstructions(domain)
}

// ------------------------------------------------------------ add domain

const hostname = ref('')
const addError = ref<string | null>(null)
const adding = ref(false)

const ADD_CONFLICTS: Record<string, string> = {
  hostname_taken: 'That hostname is already connected to a workspace.',
  reserved_hostname: 'That hostname is reserved for this installation. Use a subdomain you control instead.',
}

async function addDomain() {
  const workspaceId = ws.activeId.value
  if (!workspaceId || adding.value) return

  const value = hostname.value.trim().toLowerCase()
  if (!value) {
    addError.value = 'Enter a hostname, for example go.example.com'
    return
  }

  adding.value = true
  addError.value = null
  try {
    const created = await domains.create(workspaceId, { hostname: value })
    list.value = [created, ...list.value]
    if (created.dns_instructions) instructions.value[created.id] = created.dns_instructions
    // Straight into the records: creating the domain is step one of three.
    expanded.value[created.id] = true
    hostname.value = ''
    toast.success(`${created.hostname} added — now create its DNS records`)
  } catch (error) {
    if (error instanceof ApiError) {
      addError.value = error.field('hostname') ?? ADD_CONFLICTS[error.code] ?? error.message
    } else {
      addError.value = 'Could not add the domain.'
    }
  } finally {
    adding.value = false
  }
}

// --------------------------------------------------------------- verify

async function verify(domain: Domain) {
  const workspaceId = ws.activeId.value
  if (!workspaceId || verifying.value[domain.id]) return

  verifying.value[domain.id] = true
  try {
    // A 200 here means the check ran, not that it passed. The outcome lives in
    // status and verification_error, so never treat the response code as
    // success — that is the difference between "verified" and "we looked".
    const result = await domains.verify(workspaceId, domain.id)
    replaceDomain(result)

    if (result.status === 'active') {
      toast.success(`${result.hostname} is verified`)
    } else {
      // The error must stay on screen while the user edits their DNS panel,
      // so the row is expanded and the message rendered inline, not toasted.
      expanded.value[result.id] = true
      toast.error(`${result.hostname} is not verified yet — see the details below`)
    }
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not run the verification check.')
  } finally {
    verifying.value[domain.id] = false
  }
}

async function makeDefault(domain: Domain) {
  const workspaceId = ws.activeId.value
  if (!workspaceId || settingDefault.value[domain.id]) return

  settingDefault.value[domain.id] = true
  try {
    await domains.setDefault(workspaceId, domain.id)
    // Reload rather than patch: promoting one domain demotes another, and the
    // server is the only thing that knows which.
    await load()
    toast.success(`${domain.hostname} is now the default domain`)
  } catch (error) {
    toast.error(error instanceof ApiError ? error.message : 'Could not set the default domain.')
  } finally {
    settingDefault.value[domain.id] = false
  }
}

// --------------------------------------------------------------- delete

const deleteTarget = ref<Domain | null>(null)
const deleteOpen = ref(false)
const deleting = ref(false)
const deleteError = ref<string | null>(null)

function askDelete(domain: Domain) {
  deleteTarget.value = domain
  deleteError.value = null
  deleteOpen.value = true
}

async function confirmDelete() {
  const workspaceId = ws.activeId.value
  const target = deleteTarget.value
  if (!workspaceId || !target) return

  deleting.value = true
  deleteError.value = null
  try {
    await domains.remove(workspaceId, target.id)
    list.value = list.value.filter(d => d.id !== target.id)
    toast.success(`Removed ${target.hostname}`)
    deleteOpen.value = false
    deleteTarget.value = null
  } catch (error) {
    if (error instanceof ApiError && error.code === 'domain_has_links') {
      deleteError.value = `${target.hostname} still has short links on it. Delete those links first — removing the domain would break every URL already shared.`
    } else {
      deleteError.value = error instanceof ApiError ? error.message : 'Could not remove the domain.'
    }
  } finally {
    deleting.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <p class="mb-1 text-sm font-semibold text-(--color-accent)">Manage</p>
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
        Domains
      </h1>
      <p class="mt-0.5 text-sm text-(--color-content-muted)">
        Short links are served from the custom domains you connect here.
      </p>
    </header>

    <p class="rounded-md border border-(--color-border) bg-(--color-surface-muted) px-4 py-3 text-sm text-(--color-content-muted)">
      DNS changes can take up to an hour to propagate. If verification fails right
      after you edit your records, wait and try again before changing anything.
    </p>

    <!-- Add -->
    <UiCard
      v-if="canManage"
      title="Add a domain"
      description="Use a subdomain you control, such as go.example.com."
    >
      <form class="flex flex-wrap items-end gap-3" @submit.prevent="addDomain">
        <div class="min-w-[16rem] flex-1">
          <UiInput
            v-model="hostname"
            label="Hostname"
            placeholder="go.example.com"
            autocomplete="off"
            :disabled="adding"
            :error="addError ?? undefined"
          />
        </div>
        <UiButton type="submit" :loading="adding">
          Add domain
        </UiButton>
      </form>
    </UiCard>

    <!-- List -->
    <p v-if="loading" class="text-sm text-(--color-content-muted)" role="status">
      Loading domains…
    </p>

    <UiCard v-else-if="loadError">
      <p class="text-sm text-(--color-danger)" role="alert">
        {{ loadError }}
      </p>
      <UiButton class="mt-3" variant="secondary" size="sm" @click="load">
        Try again
      </UiButton>
    </UiCard>

    <UiCard v-else-if="!list.length" :padded="false">
      <UiEmptyState
        title="No domains connected"
        description="Connect a domain you own, add the DNS records we show you, then verify it. Links can only be created on a verified domain."
      />
    </UiCard>

    <div v-else class="flex flex-col gap-3">
      <UiCard v-for="domain in list" :key="domain.id" :padded="false">
        <div class="flex flex-wrap items-start justify-between gap-3 p-5">
          <div class="min-w-0">
            <div class="flex flex-wrap items-center gap-2">
              <h2 class="break-all font-medium">
                {{ domain.hostname }}
              </h2>
              <UiBadge v-if="domain.is_default" tone="info">
                Default
              </UiBadge>
            </div>

            <!-- Plan §39: DNS and SSL are separate states. -->
            <div class="mt-2 flex flex-wrap items-center gap-2">
              <DomainStatusBadge kind="dns" :status="domain.status" />
              <DomainStatusBadge kind="ssl" :status="domain.ssl_status" />
            </div>

            <p class="mt-2 text-xs text-(--color-content-subtle)">
              Added {{ formatDateTime(domain.created_at) }}
              <span v-if="domain.verified_at"> · verified {{ formatDateTime(domain.verified_at) }}</span>
            </p>
          </div>

          <div class="flex flex-wrap items-center gap-2">
            <UiButton variant="ghost" size="sm" @click="toggleRow(domain)">
              {{ expanded[domain.id] ? 'Hide DNS records' : 'DNS records' }}
            </UiButton>
            <UiButton
              v-if="canManage"
              :variant="domain.status === 'active' ? 'ghost' : 'secondary'"
              size="sm"
              :loading="verifying[domain.id]"
              @click="verify(domain)"
            >
              {{ domain.status === 'active' ? 'Re-check DNS' : 'Verify' }}
            </UiButton>
            <UiButton
              v-if="canManage && domain.status === 'active' && !domain.is_default"
              variant="secondary"
              size="sm"
              :loading="settingDefault[domain.id]"
              @click="makeDefault(domain)"
            >
              Set as default
            </UiButton>
            <UiButton
              v-if="canManage"
              variant="ghost"
              size="sm"
              @click="askDelete(domain)"
            >
              <span class="text-(--color-danger)">Remove</span>
            </UiButton>
          </div>
        </div>

        <!-- The verification failure stays on the row, not in a toast: the
             user reads it while editing DNS somewhere else. -->
        <div
          v-if="domain.verification_error"
          class="border-t border-(--color-border) bg-red-500/5 px-5 py-3"
          role="alert"
        >
          <p class="text-sm font-medium text-(--color-danger)">
            Verification failed
          </p>
          <p class="mt-1 whitespace-pre-line text-sm text-(--color-content)">
            {{ domain.verification_error }}
          </p>
        </div>

        <div v-if="expanded[domain.id]" class="border-t border-(--color-border) px-5 py-4">
          <p v-if="fetchingInstructions[domain.id] && !instructions[domain.id]" class="text-sm text-(--color-content-muted)" role="status">
            Loading DNS records…
          </p>
          <template v-else-if="instructions[domain.id]">
            <p class="mb-3 text-sm text-(--color-content-muted)">
              Create these records with your DNS provider, then press Verify.
            </p>
            <DomainDnsRecords :instructions="instructions[domain.id]!" />
          </template>
          <p v-else class="text-sm text-(--color-content-muted)">
            DNS records are unavailable for this domain.
          </p>
        </div>
      </UiCard>
    </div>

    <UiModal
      v-model="deleteOpen"
      title="Remove this domain?"
      description="Short links on this domain stop resolving. This cannot be undone."
      danger
    >
      <p v-if="deleteTarget" class="rounded-md bg-(--color-surface-muted) px-3 py-2 font-mono text-xs">
        {{ deleteTarget.hostname }}
      </p>
      <p v-if="deleteError" class="mt-3 text-sm text-(--color-danger)" role="alert">
        {{ deleteError }}
      </p>

      <template #actions>
        <UiButton variant="secondary" @click="deleteOpen = false">
          Cancel
        </UiButton>
        <UiButton variant="danger" :loading="deleting" @click="confirmDelete">
          Remove domain
        </UiButton>
      </template>
    </UiModal>
  </div>
</template>
