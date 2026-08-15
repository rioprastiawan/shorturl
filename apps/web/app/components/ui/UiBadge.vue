<script setup lang="ts">
type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info'

withDefaults(defineProps<{ tone?: Tone, dot?: boolean }>(), {
  tone: 'neutral',
  dot: false,
})

const tones: Record<Tone, string> = {
  neutral: 'bg-(--color-surface-muted) text-(--color-content-muted) border-(--color-border)',
  success: 'bg-emerald-500/10 text-emerald-700 dark:text-emerald-400 border-emerald-500/25',
  warning: 'bg-amber-500/10 text-amber-700 dark:text-amber-400 border-amber-500/25',
  danger: 'bg-red-500/10 text-red-700 dark:text-red-400 border-red-500/25',
  info: 'bg-blue-500/10 text-blue-700 dark:text-blue-400 border-blue-500/25',
}

const dotTones: Record<Tone, string> = {
  neutral: 'bg-slate-400',
  success: 'bg-emerald-500',
  warning: 'bg-amber-500',
  danger: 'bg-red-500',
  info: 'bg-blue-500',
}
</script>

<template>
  <span
    class="inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-xs font-medium"
    :class="tones[tone]"
  >
    <span v-if="dot" class="size-1.5 rounded-full" :class="dotTones[tone]" aria-hidden="true" />
    <slot />
  </span>
</template>
