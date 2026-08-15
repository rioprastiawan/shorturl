<script setup lang="ts">
import type { LinkStatus } from '~/types/api'

const props = defineProps<{ status: LinkStatus | string }>()

const tones: Record<string, 'success' | 'warning' | 'neutral'> = {
  active: 'success',
  disabled: 'warning',
  archived: 'neutral',
}

const labels: Record<string, string> = {
  active: 'Active',
  disabled: 'Disabled',
  archived: 'Archived',
}

const tone = computed(() => tones[props.status] ?? 'neutral')
const label = computed(() => labels[props.status] ?? props.status)
</script>

<template>
  <UiBadge :tone="tone" dot>
    {{ label }}
  </UiBadge>
</template>
