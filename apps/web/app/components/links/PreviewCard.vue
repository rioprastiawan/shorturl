<script setup lang="ts">
import type { PagePreview } from '~/types/api'

const props = withDefaults(defineProps<{
  preview: PagePreview
  destinationUrl: string
  hover?: boolean
}>(), { hover: false })

const open = ref(false)
const position = ref({ top: 0, left: 0 })

function show(event: MouseEvent | FocusEvent) {
  if (!props.hover) return
  const target = event.currentTarget as HTMLElement
  const rect = target.getBoundingClientRect()
  const width = Math.min(360, window.innerWidth - 32)
  const estimatedHeight = props.preview.image_url ? 310 : 190
  position.value = {
    left: Math.max(16, Math.min(rect.left, window.innerWidth - width - 16)),
    top: rect.bottom + estimatedHeight + 12 < window.innerHeight
      ? rect.bottom + 8
      : Math.max(16, rect.top - estimatedHeight - 8),
  }
  open.value = true
}

function hide() { open.value = false }

function hostname(raw: string): string {
  try { return new URL(raw).hostname }
  catch { return raw }
}
</script>

<template>
  <span
    v-if="hover"
    class="inline-block"
    tabindex="0"
    @mouseenter="show"
    @mouseleave="hide"
    @focus="show"
    @blur="hide"
  >
    <slot />
  </span>

  <Teleport v-if="hover" to="body">
    <Transition
      enter-active-class="transition duration-150 ease-out"
      enter-from-class="translate-y-1 opacity-0"
      leave-active-class="transition duration-100 ease-in"
      leave-to-class="translate-y-1 opacity-0"
    >
      <article
        v-if="open"
        class="pointer-events-none fixed z-50 w-[min(22.5rem,calc(100vw-2rem))] overflow-hidden rounded-xl border border-(--color-border) bg-(--color-surface-raised) shadow-2xl"
        :style="{ top: `${position.top}px`, left: `${position.left}px` }"
        role="tooltip"
      >
        <img
          v-if="preview.image_url"
          :src="preview.image_url"
          alt=""
          class="h-36 w-full bg-(--color-surface-muted) object-cover"
          loading="lazy"
          referrerpolicy="no-referrer"
        >
        <div class="p-4">
          <div class="flex items-center gap-2 text-xs text-(--color-content-muted)">
            <img
              v-if="preview.favicon_url"
              :src="preview.favicon_url"
              alt=""
              class="size-4 rounded-sm"
              loading="lazy"
              referrerpolicy="no-referrer"
            >
            <span class="truncate">{{ preview.site_name || hostname(destinationUrl) }}</span>
          </div>
          <p class="mt-2 line-clamp-2 text-sm font-semibold">{{ preview.title || destinationUrl }}</p>
          <p v-if="preview.description" class="mt-1 line-clamp-3 text-xs leading-5 text-(--color-content-muted)">
            {{ preview.description }}
          </p>
          <p class="mt-2 truncate text-xs text-(--color-content-subtle)">{{ destinationUrl }}</p>
        </div>
      </article>
    </Transition>
  </Teleport>

  <article
    v-if="!hover"
    class="overflow-hidden rounded-xl border border-(--color-border) bg-(--color-surface-muted)"
  >
    <img
      v-if="preview.image_url"
      :src="preview.image_url"
      alt=""
      class="max-h-52 w-full object-cover"
      loading="lazy"
      referrerpolicy="no-referrer"
    >
    <div class="p-4">
      <div class="flex items-center gap-2 text-xs text-(--color-content-muted)">
        <img v-if="preview.favicon_url" :src="preview.favicon_url" alt="" class="size-4 rounded-sm" referrerpolicy="no-referrer">
        <span>{{ preview.site_name || hostname(destinationUrl) }}</span>
      </div>
      <p class="mt-2 font-semibold">{{ preview.title || destinationUrl }}</p>
      <p v-if="preview.description" class="mt-1 text-sm leading-5 text-(--color-content-muted)">{{ preview.description }}</p>
    </div>
  </article>
</template>
