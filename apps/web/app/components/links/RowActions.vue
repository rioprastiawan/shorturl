<script setup lang="ts">
/**
 * The per-row "…" menu.
 *
 * A menu rather than three inline buttons because the table already carries
 * six columns, and Delete sitting one pixel from Copy is how people delete
 * links they meant to copy.
 */
const props = defineProps<{ linkId: string }>()
const emit = defineEmits<{ copy: [], delete: [] }>()

const open = ref(false)
const root = useTemplateRef<HTMLElement>('root')

onClickOutside(root, () => (open.value = false))

function choose(action: 'copy' | 'delete') {
  open.value = false
  if (action === 'copy') emit('copy')
  else emit('delete')
}

const editTo = computed(() => `/dashboard/links/${props.linkId}`)
</script>

<template>
  <div ref="root" class="relative inline-block text-left">
    <button
      type="button"
      class="rounded-md px-2 py-1 text-sm text-(--color-content-muted) transition-colors hover:bg-(--color-surface-muted) hover:text-(--color-content)"
      :aria-expanded="open"
      aria-haspopup="menu"
      aria-label="Link actions"
      @keydown.esc="open = false"
      @click="open = !open"
    >
      &hellip;
    </button>

    <div
      v-if="open"
      role="menu"
      class="absolute right-0 z-20 mt-1 w-40 overflow-hidden rounded-md border border-(--color-border) bg-(--color-surface-raised) py-1 shadow-lg"
      @keydown.esc="open = false"
    >
      <NuxtLink
        role="menuitem"
        :to="editTo"
        class="block px-3 py-1.5 text-sm text-(--color-content) hover:bg-(--color-surface-muted)"
        @click="open = false"
      >
        Edit
      </NuxtLink>
      <button
        role="menuitem"
        type="button"
        class="block w-full px-3 py-1.5 text-left text-sm text-(--color-content) hover:bg-(--color-surface-muted)"
        @click="choose('copy')"
      >
        Copy short URL
      </button>
      <button
        role="menuitem"
        type="button"
        class="block w-full px-3 py-1.5 text-left text-sm text-(--color-danger) hover:bg-(--color-surface-muted)"
        @click="choose('delete')"
      >
        Delete
      </button>
    </div>
  </div>
</template>
