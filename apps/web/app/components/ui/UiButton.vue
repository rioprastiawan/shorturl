<script setup lang="ts">
type Variant = 'primary' | 'secondary' | 'ghost' | 'danger'
type Size = 'sm' | 'md'

const props = withDefaults(defineProps<{
  variant?: Variant
  size?: Size
  type?: 'button' | 'submit' | 'reset'
  loading?: boolean
  disabled?: boolean
  to?: string
}>(), {
  variant: 'primary',
  size: 'md',
  type: 'button',
  loading: false,
  disabled: false,
  to: undefined,
})

const variants: Record<Variant, string> = {
  primary: 'bg-(--color-accent) text-(--color-accent-content) hover:bg-(--color-accent-hover) border-transparent',
  secondary: 'bg-transparent text-(--color-content) border-(--color-border-strong) hover:bg-(--color-surface-muted)',
  ghost: 'bg-transparent text-(--color-content-muted) border-transparent hover:bg-(--color-surface-muted) hover:text-(--color-content)',
  danger: 'bg-(--color-danger) text-white hover:bg-(--color-danger-hover) border-transparent',
}

const sizes: Record<Size, string> = {
  sm: 'px-2.5 py-1.5 text-xs gap-1.5',
  md: 'px-3 py-1.5 text-sm gap-1.5',
}

const classes = computed(() => [
  'inline-flex items-center justify-center rounded-md border font-semibold shadow-sm transition-all duration-200 ease-out hover:-translate-y-0.5 hover:shadow-md active:translate-y-0 active:scale-[0.98] active:shadow-sm',
  'disabled:cursor-not-allowed disabled:opacity-50',
  variants[props.variant],
  sizes[props.size],
])
</script>

<template>
  <NuxtLink v-if="to" :to="to" :class="classes">
    <slot />
  </NuxtLink>
  <button v-else :type="type" :disabled="disabled || loading" :class="classes">
    <svg
      v-if="loading"
      class="size-3.5 animate-spin"
      viewBox="0 0 24 24"
      fill="none"
      aria-hidden="true"
    >
      <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4" />
      <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.4 0 0 5.4 0 12h4z" />
    </svg>
    <slot />
  </button>
</template>
