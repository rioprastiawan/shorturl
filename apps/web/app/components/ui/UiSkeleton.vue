<script setup lang="ts">
withDefaults(defineProps<{
  width?: string
  height?: string
  rounded?: 'sm' | 'md' | 'lg' | 'full'
}>(), {
  width: '100%',
  height: '1rem',
  rounded: 'md',
})

const radii = {
  sm: 'rounded',
  md: 'rounded-md',
  lg: 'rounded-xl',
  full: 'rounded-full',
}
</script>

<template>
  <span
    aria-hidden="true"
    class="skeleton-shimmer block max-w-full bg-(--color-border)"
    :class="radii[rounded]"
    :style="{ width, height }"
  />
</template>
