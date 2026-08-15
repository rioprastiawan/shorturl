<script setup lang="ts">
withDefaults(defineProps<{
  rows?: number
  columns?: number
}>(), { rows: 5, columns: 4 })
</script>

<template>
  <div aria-hidden="true" class="overflow-hidden">
    <div class="flex gap-6 border-b border-(--color-border) px-5 py-3">
      <UiSkeleton v-for="column in columns" :key="`head-${column}`" height="0.65rem" :width="column === 1 ? '28%' : '15%'" />
    </div>
    <div v-for="row in rows" :key="row" class="flex items-center gap-6 border-b border-(--color-border) px-5 py-4 last:border-0">
      <UiSkeleton v-for="column in columns" :key="column" height="0.85rem" :width="column === 1 ? '28%' : column === columns ? '10%' : '15%'" />
    </div>
  </div>
</template>
