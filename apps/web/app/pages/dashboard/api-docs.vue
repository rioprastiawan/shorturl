<script setup lang="ts">
import spec from '../../../public/openapi.json'

definePageMeta({ middleware: ['auth', 'workspace-admin'] })
useHead({ title: 'API documentation · ShortURL' })

type Method = 'get' | 'post' | 'patch' | 'delete'
type Operation = {
  summary: string
  description?: string
  operationId: string
  responses: Record<string, { $ref?: string, description?: string }>
}

const methods: Method[] = ['get', 'post', 'patch', 'delete']
const search = ref('')
const selectedId = ref('createLink')
const errorExamplesOpen = ref(false)

const operations = Object.entries(spec.paths).flatMap(([path, pathItem]) =>
  methods.flatMap((method) => {
    const operation = (pathItem as Record<string, unknown>)[method] as Operation | undefined
    return operation ? [{ method, path, ...operation }] : []
  }),
)

const filtered = computed(() => {
  const query = search.value.trim().toLowerCase()
  if (!query) return operations
  return operations.filter(item => `${item.method} ${item.path} ${item.summary} ${item.description}`.toLowerCase().includes(query))
})

const selected = computed(() => operations.find(item => item.operationId === selectedId.value) ?? operations[0])
const linkExample = spec.components.examples.Link.value
const listExample = spec.components.examples.LinkList.value
const requestExample = spec.components.examples.CreateLink.value
const validationExample = spec.components.examples.Validation.value
const unauthorizedExample = spec.components.examples.Unauthorized.value

const responseExample = computed(() => selected.value?.operationId === 'listLinks'
  ? listExample
  : selected.value?.method === 'delete'
    ? null
    : { data: linkExample })

const requestBodyExample = computed(() => {
  if (selected.value?.operationId === 'createLink') return requestExample
  if (selected.value?.operationId === 'updateLink') return { title: 'Updated campaign', expires_at: null, status: 'active' }
  return null
})

const responseCodes = computed(() => Object.entries(selected.value?.responses ?? {}).map(([code, response]) => {
  const refName = response.$ref?.split('/').at(-1) as keyof typeof spec.components.responses | undefined
  return { code, description: refName ? spec.components.responses[refName].description : response.description }
}))

const methodClass: Record<Method, string> = {
  get: 'bg-sky-500/12 text-sky-700 dark:text-sky-300',
  post: 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-300',
  patch: 'bg-amber-500/12 text-amber-700 dark:text-amber-300',
  delete: 'bg-red-500/12 text-red-700 dark:text-red-300',
}

const pretty = (value: unknown) => JSON.stringify(value, null, 2)
</script>

<template>
  <div class="mx-auto flex w-full max-w-[92rem] flex-col gap-4">
    <header class="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <div>
        <div class="mb-1 flex items-center gap-2 text-xs font-semibold text-(--color-accent)">
          <Icon name="lucide:braces" size="15" />
          OpenAPI {{ spec.openapi }}
        </div>
        <h1 class="text-xl font-bold tracking-tight sm:text-2xl">API documentation</h1>
        <p class="mt-1 max-w-2xl text-sm text-(--color-content-muted)">
          Explore every endpoint, request field, response shape, and error without guessing the backend contract.
        </p>
      </div>
      <div class="grid w-full grid-cols-2 gap-2 sm:flex sm:w-auto sm:shrink-0">
        <UiButton to="/dashboard/api-keys" variant="secondary">
          <Icon name="lucide:key-round" size="15" /> API keys
        </UiButton>
        <a href="/openapi.json" download="shorturl-openapi.json" class="inline-flex items-center gap-1.5 rounded-md border border-(--color-border-strong) px-3 py-1.5 text-sm font-semibold shadow-sm transition-all hover:-translate-y-0.5 hover:bg-(--color-surface-muted)">
          <Icon name="lucide:download" size="15" /> OpenAPI file
        </a>
      </div>
    </header>

    <section class="grid gap-3 sm:grid-cols-3">
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-3.5">
        <p class="text-[11px] font-bold uppercase tracking-wider text-(--color-content-subtle)">Base URL</p>
        <code class="mt-1.5 block text-xs font-semibold text-(--color-content)">/api/v1</code>
      </div>
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-3.5">
        <p class="text-[11px] font-bold uppercase tracking-wider text-(--color-content-subtle)">Authentication</p>
        <code class="mt-1.5 block text-xs font-semibold text-(--color-content)">Bearer shr_live_...</code>
      </div>
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-3.5">
        <p class="text-[11px] font-bold uppercase tracking-wider text-(--color-content-subtle)">Response format</p>
        <code class="mt-1.5 block text-xs font-semibold text-(--color-content)">application/json</code>
      </div>
    </section>

    <div class="grid min-h-[38rem] overflow-hidden rounded-2xl border border-(--color-border) bg-(--color-surface-raised) lg:grid-cols-[18rem_minmax(0,1fr)]">
      <aside class="border-b border-(--color-border) bg-(--color-surface-muted)/45 p-3 lg:border-r lg:border-b-0">
        <label class="relative block">
          <Icon name="lucide:search" size="15" class="absolute top-1/2 left-3 -translate-y-1/2 text-(--color-content-subtle)" />
          <input v-model="search" type="search" placeholder="Search endpoints" class="h-9 w-full rounded-lg border border-(--color-border) bg-(--color-surface-raised) pr-3 pl-9 text-xs outline-none transition focus:border-(--color-accent) focus:ring-2 focus:ring-(--color-accent)/15">
        </label>
        <nav class="mt-3 grid max-h-52 gap-1 overflow-y-auto lg:max-h-[33rem]" aria-label="API endpoints">
          <button
            v-for="item in filtered"
            :key="item.operationId"
            type="button"
            class="flex items-center gap-2 rounded-lg px-2.5 py-2 text-left transition-colors"
            :class="selected?.operationId === item.operationId ? 'bg-(--color-surface-raised) shadow-sm ring-1 ring-(--color-border)' : 'hover:bg-(--color-surface-raised)/70'"
            @click="selectedId = item.operationId"
          >
            <span class="w-12 shrink-0 rounded px-1.5 py-0.5 text-center font-mono text-[9px] font-black uppercase" :class="methodClass[item.method]">{{ item.method }}</span>
            <span class="min-w-0">
              <span class="block truncate text-xs font-semibold">{{ item.summary }}</span>
              <code class="block truncate text-[10px] text-(--color-content-subtle)">{{ item.path }}</code>
            </span>
          </button>
        </nav>
      </aside>

      <main v-if="selected" class="min-w-0 p-4 sm:p-5">
        <div class="flex flex-wrap items-center gap-2">
          <span class="rounded px-2 py-1 font-mono text-[10px] font-black uppercase" :class="methodClass[selected.method]">{{ selected.method }}</span>
          <code class="min-w-0 break-all text-sm font-semibold">{{ selected.path }}</code>
        </div>
        <h2 class="mt-3 text-lg font-bold">{{ selected.summary }}</h2>
        <p class="mt-1 max-w-3xl text-sm leading-relaxed text-(--color-content-muted)">{{ selected.description }}</p>

        <div class="mt-5 grid min-w-0 max-w-full gap-4 [&>*]:min-w-0 [&>*]:max-w-full xl:grid-cols-2">
          <section v-if="requestBodyExample" class="min-w-0 max-w-full overflow-hidden">
            <h3 class="mb-2 text-xs font-bold uppercase tracking-wider text-(--color-content-subtle)">Example request body</h3>
            <DeveloperCodeBlock :code="pretty(requestBodyExample)" language="json" />
          </section>
          <section class="min-w-0 max-w-full overflow-hidden" :class="requestBodyExample ? '' : 'xl:col-span-2'">
            <h3 class="mb-2 text-xs font-bold uppercase tracking-wider text-(--color-content-subtle)">Example success response</h3>
            <div v-if="responseExample" class="min-w-0 max-w-full"><DeveloperCodeBlock :code="pretty(responseExample)" language="json" /></div>
            <div v-else class="grid min-h-24 place-items-center rounded-xl border border-dashed border-(--color-border) bg-(--color-surface-muted)/50 text-sm text-(--color-content-muted)">
              204 No Content — the response body is empty.
            </div>
          </section>
        </div>

        <section class="mt-5">
          <h3 class="mb-2 text-xs font-bold uppercase tracking-wider text-(--color-content-subtle)">Possible responses</h3>
          <div class="flex flex-wrap gap-2">
            <span v-for="response in responseCodes" :key="response.code" class="inline-flex items-center gap-2 rounded-lg border border-(--color-border) bg-(--color-surface-muted)/45 px-2.5 py-1.5 text-xs">
              <code class="font-bold" :class="Number(response.code) < 300 ? 'text-emerald-600 dark:text-emerald-400' : 'text-amber-700 dark:text-amber-300'">{{ response.code }}</code>
              <span class="text-(--color-content-muted)">{{ response.description }}</span>
            </span>
          </div>
        </section>

        <UiDisclosure v-model="errorExamplesOpen" class="mt-5" title="Authentication and validation errors" description="Inspect representative API error response bodies." icon="lucide:triangle-alert">
          <div class="grid min-w-0 max-w-full gap-3 [&>*]:min-w-0 [&>*]:max-w-full xl:grid-cols-2">
            <DeveloperCodeBlock :code="pretty(unauthorizedExample)" language="json" />
            <DeveloperCodeBlock :code="pretty(validationExample)" language="json" />
          </div>
        </UiDisclosure>
      </main>
    </div>
  </div>
</template>
