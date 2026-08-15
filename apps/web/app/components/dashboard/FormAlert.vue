<script setup lang="ts">
/**
 * A form-level message for errors that belong to no single input — a 401 on
 * login, a 409 on delete. Field-level errors go under their input instead.
 *
 * role="alert" so the failure is announced rather than silently appearing
 * above a form the user is still looking down at.
 */
type Tone = 'danger' | 'warning' | 'info'

withDefaults(defineProps<{ tone?: Tone }>(), { tone: 'danger' })

const tones: Record<Tone, string> = {
  danger: 'border-red-500/30 bg-red-500/10 text-red-700 dark:text-red-400',
  warning: 'border-amber-500/30 bg-amber-500/10 text-amber-700 dark:text-amber-400',
  info: 'border-blue-500/30 bg-blue-500/10 text-blue-700 dark:text-blue-400',
}
</script>

<template>
  <p class="rounded-md border px-3 py-2 text-sm" :class="tones[tone]" role="alert">
    <slot />
  </p>
</template>
