<script setup lang="ts">
const props = withDefaults(defineProps<{
  title: string
  description?: string
  /** Renders the confirm button in the destructive style. */
  danger?: boolean
  size?: 'md' | 'lg'
}>(), {
  description: undefined,
  danger: false,
  size: 'md',
})

const open = defineModel<boolean>({ default: false })
const dialog = useTemplateRef<HTMLDialogElement>('dialog')
let closeTimer: ReturnType<typeof setTimeout> | undefined

function finishClose() {
  if (closeTimer) clearTimeout(closeTimer)
  closeTimer = undefined
  const el = dialog.value
  if (!el) return
  delete el.dataset.closing
  if (el.open) el.close()
}

// A native <dialog> gives focus trapping, Escape handling, inert background,
// and the top layer for free — all of which are easy to get wrong by hand.
watch(open, (isOpen) => {
  const el = dialog.value
  if (!el) return
  if (isOpen) {
    if (closeTimer) clearTimeout(closeTimer)
    closeTimer = undefined
    delete el.dataset.closing
    if (!el.open) el.showModal()
    return
  }
  if (el.open && !el.dataset.closing) {
    el.dataset.closing = 'true'
    // animationend normally closes it; this is a fallback if animations are
    // disabled or interrupted by the browser.
    closeTimer = setTimeout(finishClose, 220)
  }
})

onBeforeUnmount(() => {
  if (closeTimer) clearTimeout(closeTimer)
})

function onClose() {
  if (closeTimer) clearTimeout(closeTimer)
  closeTimer = undefined
  if (dialog.value) delete dialog.value.dataset.closing
  open.value = false
}

function onCancel(event: Event) {
  event.preventDefault()
  open.value = false
}

function onAnimationEnd(event: AnimationEvent) {
  if (event.target === dialog.value && dialog.value?.dataset.closing) finishClose()
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
    class="m-auto max-h-[calc(100dvh-1.5rem)] w-[calc(100vw-1.5rem)] overflow-y-auto rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-0 text-(--color-content) shadow-2xl backdrop:bg-black/40 backdrop:backdrop-blur-[2px]"
    :class="size === 'lg' ? 'max-w-3xl' : 'max-w-md'"
    @animationend="onAnimationEnd"
    @cancel="onCancel"
    @close="onClose"
    @click="onClick"
  >
    <div class="p-3.5 sm:p-4">
      <h2 class="font-medium">
        {{ title }}
      </h2>
      <p v-if="description" class="mt-1 text-sm text-(--color-content-muted)">
        {{ description }}
      </p>

      <div v-if="$slots.default" class="mt-3">
        <slot />
      </div>

      <div class="mt-4 flex flex-wrap justify-end gap-2">
        <slot name="actions">
          <UiButton variant="secondary" @click="open = false">
            Cancel
          </UiButton>
        </slot>
      </div>
    </div>
  </dialog>
</template>
