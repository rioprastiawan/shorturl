<script setup lang="ts">
definePageMeta({ middleware: ['auth', 'redirect-create-link'] })
useHead({ title: 'Create link · ShortURL' })
</script>

<template>
  <div class="mx-auto w-full max-w-2xl space-y-4" role="status" aria-label="Opening link creator">
    <UiSkeleton height="1.5rem" width="35%" />
    <UiSkeleton height="0.75rem" width="65%" />
    <UiSkeleton height="16rem" rounded="lg" />
  </div>
</template>
