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
    class="flex flex-wrap items-center justify-between gap-3 border-t border-(--color-border) px-5 py-3"
  >
    <p class="text-xs text-(--color-content-muted)" aria-live="polite">
      Page {{ page }}<span v-if="label"> · {{ label }}</span>
    </p>

    <nav class="flex items-center gap-2" aria-label="Pagination">
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="page <= 1 || loading"
        aria-label="Previous page"
        @click="emit('previous')"
      >
        <Icon name="lucide:chevron-left" size="14" />
        Previous
      </UiButton>
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="!hasNext || loading"
        :loading="loading"
        aria-label="Next page"
        @click="emit('next')"
      >
        Next
        <Icon v-if="!loading" name="lucide:chevron-right" size="14" />
      </UiButton>
    </nav>
  </div>
</template>
