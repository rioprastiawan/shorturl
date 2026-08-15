<script setup lang="ts">
definePageMeta({ middleware: ['auth', 'redirect-create-link'] })
useHead({ title: 'Create link · ShortURL' })
</script>

<template>
  <p class="text-sm text-(--color-content-muted)" role="status">Opening link creator…</p>
</template>
