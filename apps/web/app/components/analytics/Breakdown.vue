<script setup lang="ts">
import type { ValueStat } from '~/types/api'
import { formatNumber } from '~/components/links/format'

/**
 * One dimension as a horizontal bar list. Bars are proportional to the largest
 * value in this card, not to the workspace total, so a card whose top value is
 * 4 clicks is still readable.
 */
const props = withDefaults(defineProps<{
  title: string
  items: ValueStat[]
  limit?: number
  emptyLabel?: string
}>(), {
  limit: 8,
  emptyLabel: 'Direct / none',
})

const rows = computed(() => props.items.slice(0, props.limit))
const max = computed(() => rows.value.reduce((acc, r) => Math.max(acc, r.clicks), 0))

function width(clicks: number): string {
  if (max.value <= 0) return '0%'
  return `${Math.max(2, (clicks / max.value) * 100)}%`
}
</script>

<template>
  <UiCard :title="title">
    <ul class="flex flex-col gap-2.5">
      <li v-for="row in rows" :key="row.value" class="flex flex-col gap-1">
        <div class="flex items-baseline justify-between gap-3 text-sm">
          <span class="min-w-0 truncate" :title="row.value || emptyLabel">
            {{ row.value || emptyLabel }}
          </span>
          <span class="shrink-0 tabular-nums text-(--color-content-muted)">
            {{ formatNumber(row.clicks) }}
          </span>
        </div>
        <div class="h-1.5 w-full overflow-hidden rounded-full bg-(--color-surface-muted)">
          <div class="h-full rounded-full bg-(--color-accent)" :style="{ width: width(row.clicks) }" />
        </div>
      </li>
    </ul>
  </UiCard>
</template>
