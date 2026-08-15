<script setup lang="ts">
/**
 * The per-row "…" menu.
 *
 * A menu rather than three inline buttons because the table already carries
 * six columns, and Delete sitting one pixel from Copy is how people delete
 * links they meant to copy.
 */
const props = defineProps<{
  linkId: string
  status: 'active' | 'disabled' | 'archived'
  disabled?: boolean
}>()
const emit = defineEmits<{ copy: [], qr: [], toggle: [], delete: [] }>()

const open = ref(false)
const root = useTemplateRef<HTMLElement>('root')
const trigger = useTemplateRef<HTMLElement>('trigger')
const menu = useTemplateRef<HTMLElement>('menu')
const { floatingStyle } = useFloatingPanel(trigger, open, {
  align: 'end',
  width: 176,
  estimatedHeight: 204,
})

onClickOutside([root, menu], () => (open.value = false))

function choose(action: 'copy' | 'qr' | 'toggle' | 'delete') {
  open.value = false
  if (action === 'copy') emit('copy')
  else if (action === 'qr') emit('qr')
  else if (action === 'toggle') emit('toggle')
  else emit('delete')
}

const editTo = computed(() => `/dashboard/links/${props.linkId}`)
</script>

<template>
  <div ref="root" class="relative inline-block text-left">
    <button
      ref="trigger"
      type="button"
      class="rounded-md px-2 py-1 text-sm text-(--color-content-muted) transition-colors hover:bg-(--color-surface-muted) hover:text-(--color-content)"
      :aria-expanded="open"
      aria-haspopup="menu"
      aria-label="Link actions"
      :disabled="disabled"
      @keydown.esc="open = false"
      @click="open = !open"
    >
      &hellip;
    </button>

    <Teleport to="body">
      <Transition name="menu-down">
        <div
          v-if="open"
          ref="menu"
          role="menu"
          :style="floatingStyle"
          class="overflow-hidden rounded-xl border border-(--color-border) bg-(--color-surface-raised) py-1.5 text-(--color-content) shadow-xl"
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
        class="block w-full px-3 py-1.5 text-left text-sm text-(--color-content) hover:bg-(--color-surface-muted)"
        @click="choose('qr')"
      >
        Show QR code
      </button>
      <button
        role="menuitem"
        type="button"
        class="block w-full px-3 py-1.5 text-left text-sm text-(--color-content) hover:bg-(--color-surface-muted)"
        @click="choose('toggle')"
      >
        {{ status === 'active' ? 'Disable link' : 'Enable link' }}
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
      </Transition>
    </Teleport>
  </div>
</template>
