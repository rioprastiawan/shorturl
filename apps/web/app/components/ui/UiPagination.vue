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
    class="flex flex-wrap items-center justify-between gap-3 border-t border-(--color-border) px-5 py-3"
  >
    <p class="text-xs text-(--color-content-muted)" aria-live="polite">
      Showing {{ start }}–{{ end }} of {{ total }} {{ label }}
    </p>

    <div class="flex items-center gap-2" :aria-label="`${label} pagination`">
      <UiButton
        variant="secondary"
        size="sm"
        :disabled="page <= 1"
        :aria-label="`Previous ${label} page`"
        @click="goTo(page - 1)"
      >
        <Icon name="lucide:chevron-left" size="14" />
        Previous
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
        Next
        <Icon name="lucide:chevron-right" size="14" />
      </UiButton>
    </div>
  </div>
</template>
