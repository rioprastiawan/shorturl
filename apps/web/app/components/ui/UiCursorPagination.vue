<script setup lang="ts">
defineProps<{
  page: number
  hasNext: boolean
  loading?: boolean
  label?: string
}>()

const emit = defineEmits<{
  previous: []
  next: []
}>()
</script>

<template>
  <div
    v-if="page > 1 || hasNext"
    class="flex min-w-0 flex-col gap-2 border-t border-(--color-border) px-3.5 py-3 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between sm:gap-3 sm:px-5"
  >
    <p class="text-xs text-(--color-content-muted)" aria-live="polite">
      <span v-if="label">{{ label }}</span><span v-else>Showing current page</span>
      <span> · Page {{ page }}</span>
    </p>

    <nav class="flex w-full items-center justify-between gap-2 sm:w-auto sm:justify-start" aria-label="Pagination">
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="page <= 1 || loading"
        aria-label="Previous page"
        @click="emit('previous')"
      >
        <Icon name="lucide:chevron-left" size="14" />
        <span>Previous</span>
      </UiButton>
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="!hasNext || loading"
        :loading="loading"
        aria-label="Next page"
        @click="emit('next')"
      >
        <span>Next</span>
        <Icon v-if="!loading" name="lucide:chevron-right" size="14" />
      </UiButton>
    </nav>
  </div>
</template>
