<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  description?: string
}>(), {
  description: undefined,
})
</script>

<template>
  <div class="flex flex-col items-center gap-2.5 px-5 py-10 text-center">
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
