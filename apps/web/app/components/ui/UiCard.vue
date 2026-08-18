<script setup lang="ts">
withDefaults(defineProps<{
  title?: string
  description?: string
  padded?: boolean
  compact?: boolean
}>(), {
  title: undefined,
  description: undefined,
  padded: true,
  compact: true,
})
</script>

<template>
  <section class="motion-card-in rounded-xl border border-(--color-border) bg-(--color-surface-raised) shadow-[0_1px_2px_rgba(16,48,63,0.03)] transition-[border-color,box-shadow,transform] duration-300 ease-out hover:border-(--color-border-strong) hover:shadow-[0_8px_30px_rgba(16,48,63,0.07)]">
    <header
      v-if="title || $slots.actions"
      class="flex flex-wrap items-start justify-between border-b border-(--color-border)"
      :class="compact ? 'gap-3 px-3.5 py-3 sm:px-4' : 'gap-4 px-4 py-3.5 sm:px-5'"
    >
      <div class="min-w-0">
        <h2 v-if="title" class="font-semibold tracking-tight">
          {{ title }}
        </h2>
        <p v-if="description" class="mt-0.5 text-sm text-(--color-content-muted)">
          {{ description }}
        </p>
      </div>
      <div v-if="$slots.actions" class="max-w-full shrink-0">
        <slot name="actions" />
      </div>
    </header>

    <div :class="padded ? compact ? 'p-3.5 sm:p-4' : 'p-4 sm:p-5' : ''">
      <slot />
    </div>
  </section>
</template>
