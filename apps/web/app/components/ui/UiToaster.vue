<script setup lang="ts">
const { toasts, dismiss } = useToast()

const tones = {
  success: 'border-emerald-500/30 bg-emerald-500/10 text-emerald-800 dark:text-emerald-300',
  error: 'border-red-500/30 bg-red-500/10 text-red-800 dark:text-red-300',
  info: 'border-(--color-border) bg-(--color-surface-raised)',
}
</script>

<template>
  <!-- aria-live so a screen reader announces the result of an action that has
       no other visible outcome, such as copying a short URL. -->
  <div
    class="pointer-events-none fixed bottom-4 right-4 z-50 flex w-full max-w-sm flex-col gap-2"
    role="status"
    aria-live="polite"
  >
    <TransitionGroup
      enter-active-class="transition duration-300 ease-out"
      enter-from-class="translate-x-4 scale-95 opacity-0"
      leave-active-class="transition duration-200 ease-in"
      leave-to-class="translate-x-3 scale-95 opacity-0"
      move-class="transition-transform duration-200"
    >
      <div
        v-for="toast in toasts"
        :key="toast.id"
        class="pointer-events-auto flex items-start gap-3 rounded-lg border px-4 py-3 text-sm shadow-lg backdrop-blur"
        :class="tones[toast.tone]"
      >
        <span class="flex-1">{{ toast.message }}</span>
        <button
          class="shrink-0 opacity-60 transition-opacity hover:opacity-100"
          aria-label="Dismiss"
          @click="dismiss(toast.id)"
        >
          &times;
        </button>
      </div>
    </TransitionGroup>
  </div>
</template>
