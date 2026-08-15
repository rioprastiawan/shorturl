<script setup lang="ts">
import { ApiError } from '~/composables/useApi'

withDefaults(defineProps<{
  block?: boolean
  compact?: boolean
}>(), {
  block: false,
  compact: false,
})

type DemoSize = 'starter' | 'busy' | 'five_year'

const session = useSession()
const emit = defineEmits<{ created: [] }>()
const ws = useWorkspaces()
const toast = useToast()
const open = ref(false)
const creating = ref(false)
const size = ref<DemoSize>('starter')
const errorMessage = ref<string>()

const options = computed(() => [
  { value: 'starter', label: 'Starter — 15 links, 90 days' },
  ...(session.user.value?.is_admin
    ? [
        { value: 'busy', label: 'Busy — 500 links, 1 year' },
        { value: 'five_year', label: 'Stress test — 2,500 links, 5 years' },
      ]
    : []),
])

const details = computed(() => ({
  starter: 'A quick product tour with example campaigns, click trends, devices, browsers, referrers, and recent activity.',
  busy: 'A larger workspace for checking pagination, filtering, search, charts, and dashboard density.',
  five_year: 'An admin-only stress dataset for reviewing long-term analytics and thousands of short links.',
}[size.value]))

function show() {
  size.value = 'starter'
  errorMessage.value = undefined
  open.value = true
}

async function create() {
  if (creating.value) return
  creating.value = true
  errorMessage.value = undefined
  try {
    await ws.createDemo(size.value)
    open.value = false
    emit('created')
    toast.success('Demo workspace created')
    await navigateTo('/dashboard')
  } catch (error) {
    errorMessage.value = error instanceof ApiError ? error.message : 'Could not create the demo workspace.'
  } finally {
    creating.value = false
  }
}
</script>

<template>
  <button
    type="button"
    class="inline-flex items-center justify-center gap-2 rounded-lg font-semibold transition-colors"
    :class="[
      block ? 'w-full' : '',
      compact
        ? 'px-2 py-1.5 text-left text-sm text-(--color-content-muted) hover:bg-(--color-surface-muted) hover:text-(--color-content)'
        : 'border border-(--color-border-strong) bg-(--color-surface-raised) px-4 py-2 text-sm shadow-sm hover:bg-(--color-surface-muted)',
    ]"
    @click="show"
  >
    <Icon name="lucide:flask-conical" size="17" />
    Create demo workspace
  </button>

  <UiModal v-model="open" title="Create a demo workspace" description="Explore ShortURL with realistic sample links and analytics—no setup required.">
    <div class="flex flex-col gap-4">
      <DashboardFormAlert v-if="errorMessage">{{ errorMessage }}</DashboardFormAlert>
      <UiSelect v-model="size" label="Dataset size" :options="options" :disabled="creating" />
      <div class="rounded-xl border border-(--color-border) bg-(--color-surface-muted) p-3">
        <p class="text-sm text-(--color-content-muted)">{{ details }}</p>
      </div>
      <p class="text-xs text-(--color-content-subtle)">Demo short URLs use an example domain. Replace it with a verified domain before sharing publicly.</p>
    </div>
    <template #actions>
      <UiButton variant="secondary" :disabled="creating" @click="open = false">Cancel</UiButton>
      <UiButton :loading="creating" @click="create"><Icon name="lucide:sparkles" size="16" /> Create demo</UiButton>
    </template>
  </UiModal>
</template>
