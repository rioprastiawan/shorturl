<script setup lang="ts">
const props = withDefaults(defineProps<{
  page: number
  total: number
  pageSize?: number
  label?: string
}>(), {
  pageSize: 10,
  label: 'items',
})

const emit = defineEmits<{
  'update:page': [page: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))
const start = computed(() => props.total === 0 ? 0 : ((props.page - 1) * props.pageSize) + 1)
const end = computed(() => Math.min(props.page * props.pageSize, props.total))

function goTo(page: number) {
  emit('update:page', Math.min(Math.max(page, 1), totalPages.value))
}
</script>

<template>
  <div
    v-if="total > pageSize"
    class="flex min-w-0 flex-col gap-2 border-t border-(--color-border) px-3.5 py-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3 sm:px-5"
  >
    <p class="text-xs text-(--color-content-muted)" aria-live="polite">
      Showing {{ start }}–{{ end }} of {{ total }} {{ label }}
    </p>

    <nav class="flex w-full items-center justify-between gap-2 sm:w-auto sm:justify-start" :aria-label="`${label} pagination`">
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="page <= 1"
        :aria-label="`Previous ${label} page`"
        @click="goTo(page - 1)"
      >
        <Icon name="lucide:chevron-left" size="14" />
        <span>Previous</span>
      </UiButton>
      <span class="min-w-16 text-center text-xs font-medium text-(--color-content-muted)">
        {{ page }} / {{ totalPages }}
      </span>
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="page >= totalPages"
        :aria-label="`Next ${label} page`"
        @click="goTo(page + 1)"
      >
        <span>Next</span>
        <Icon name="lucide:chevron-right" size="14" />
      </UiButton>
    </nav>
  </div>
</template>
