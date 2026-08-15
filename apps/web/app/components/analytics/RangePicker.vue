<script setup lang="ts">
import type { PresetRange } from './types'
import { RANGE_OPTIONS } from './types'

defineProps<{ disabled?: boolean }>()

const model = defineModel<PresetRange>({ required: true })
</script>

<template>
  <div
    class="inline-flex flex-wrap rounded-md border border-(--color-border-strong) p-0.5"
    role="group"
    aria-label="Date range"
  >
    <button
      v-for="option in RANGE_OPTIONS"
      :key="option.value"
      type="button"
      :disabled="disabled"
      class="rounded px-2.5 py-1 text-sm font-medium transition-colors disabled:opacity-50"
      :class="model === option.value
        ? 'bg-(--color-accent) text-(--color-accent-content)'
        : 'text-(--color-content-muted) hover:bg-(--color-surface-muted) hover:text-(--color-content)'"
      :aria-pressed="model === option.value"
      @click="model = option.value"
    >
      {{ option.label }}
    </button>
  </div>
</template>
