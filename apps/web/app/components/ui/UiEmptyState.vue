<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  description?: string
}>(), {
  description: undefined,
})
</script>

<template>
  <div class="flex flex-col items-center gap-3 px-6 py-14 text-center">
    <h3 class="font-medium">
      {{ title }}
    </h3>
    <p v-if="description" class="max-w-sm text-sm text-(--color-content-muted)">
      {{ description }}
    </p>
    <div v-if="$slots.default" class="mt-1">
      <slot />
    </div>
  </div>
</template>
