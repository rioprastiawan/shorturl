<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  description?: string
  padded?: boolean
}>(), {
  title: undefined,
  description: undefined,
  padded: true,
})
</script>

<template>
  <section class="rounded-lg border border-(--color-border) bg-(--color-surface-raised)">
    <header
      v-if="title || $slots.actions"
      class="flex items-start justify-between gap-4 border-b border-(--color-border) px-5 py-4"
    >
      <div class="min-w-0">
        <h2 v-if="title" class="font-medium">
          {{ title }}
        </h2>
        <p v-if="description" class="mt-0.5 text-sm text-(--color-content-muted)">
          {{ description }}
        </p>
      </div>
      <div v-if="$slots.actions" class="shrink-0">
        <slot name="actions" />
      </div>
    </header>

    <div :class="padded ? 'p-5' : ''">
      <slot />
    </div>
  </section>
</template>
