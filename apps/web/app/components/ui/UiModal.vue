<script setup lang="ts">
const props = withDefaults(defineProps<{
  title: string
  description?: string
  /** Renders the confirm button in the destructive style. */
  danger?: boolean
}>(), {
  description: undefined,
  danger: false,
})

const open = defineModel<boolean>({ default: false })
const dialog = useTemplateRef<HTMLDialogElement>('dialog')

// A native <dialog> gives focus trapping, Escape handling, inert background,
// and the top layer for free — all of which are easy to get wrong by hand.
watch(open, (isOpen) => {
  const el = dialog.value
  if (!el) return
  if (isOpen && !el.open) el.showModal()
  if (!isOpen && el.open) el.close()
})

function onClose() {
  open.value = false
}

// Clicking the backdrop closes. The dialog element itself covers only the
// panel, so a click whose target is the dialog is a click outside the content.
function onClick(event: MouseEvent) {
  if (event.target === dialog.value) open.value = false
}

defineExpose({ danger: props.danger })
</script>

<template>
  <dialog
    ref="dialog"
    class="m-auto w-[calc(100vw-2rem)] max-w-md rounded-lg border border-(--color-border) bg-(--color-surface-raised) p-0 text-(--color-content) backdrop:bg-black/40"
    @close="onClose"
    @click="onClick"
  >
    <div class="p-5">
      <h2 class="font-medium">
        {{ title }}
      </h2>
      <p v-if="description" class="mt-1 text-sm text-(--color-content-muted)">
        {{ description }}
      </p>

      <div v-if="$slots.default" class="mt-4">
        <slot />
      </div>

      <div class="mt-6 flex justify-end gap-2">
        <slot name="actions">
          <UiButton variant="secondary" @click="open = false">
            Cancel
          </UiButton>
        </slot>
      </div>
    </div>
  </dialog>
</template>
