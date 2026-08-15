<script setup lang="ts">
/**
 * One number and its label. Deliberately without a sparkline: the overview is
 * a status check, and a trend line here would compete with the analytics page
 * that exists to show trends properly.
 */
withDefaults(defineProps<{
  label: string
  value: number | string
  /** Draws attention to a value that is blocking the operator, e.g. 0 domains. */
  tone?: 'default' | 'warning'
}>(), {
  tone: 'default',
})
</script>

<template>
  <div class="motion-card-in rounded-xl border border-(--color-border) bg-(--color-surface-raised) px-4 py-3.5 shadow-[0_1px_2px_rgba(16,48,63,0.03)] transition-[border-color,box-shadow,transform] duration-200 hover:-translate-y-0.5 hover:border-(--color-border-strong) hover:shadow-md">
    <p
      class="text-2xl font-bold tabular-nums tracking-tight"
      :class="tone === 'warning' ? 'text-(--color-warning)' : ''"
    >
      {{ value }}
    </p>
    <p class="mt-0.5 text-xs font-medium text-(--color-content-muted)">
      {{ label }}
    </p>
  </div>
</template>
