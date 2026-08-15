<script setup lang="ts">
withDefaults(defineProps<{
  align?: 'left' | 'center' | 'right'
  muted?: boolean
  nowrap?: boolean
}>(), {
  align: 'left',
  muted: false,
  nowrap: false,
})
</script>

<template>
  <td
    class="px-5 py-3 align-middle"
    :class="[
      align === 'right' ? 'text-right' : align === 'center' ? 'text-center' : 'text-left',
      muted ? 'text-(--color-content-muted)' : '',
      nowrap ? 'whitespace-nowrap' : '',
    ]"
  >
    <slot />
  </td>
</template>
