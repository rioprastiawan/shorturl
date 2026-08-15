<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
useHead({ title: 'Appearance · ShortURL' })
</script>

<template>
  <div class="flex flex-col gap-6">
    <header>
      <p class="mb-1 text-sm font-semibold text-(--color-accent)">Personalization</p>
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Appearance</h1>
      <p class="text-sm text-(--color-content-muted)">Personal browser-only themes. These settings do not affect teammates or system-wide whitelabeling.</p>
    </header>
    <UiCard title="Color mode" description="Choose when the Light and Dark palettes are used.">
      <AppearancePicker />
      <p class="mt-3 text-xs text-(--color-content-muted)">System follows your device. A custom theme automatically switches to its matching palette.</p>
    </UiCard>
    <UiCard title="Theme library" description="Create named Light and Dark palette pairs stored only in this browser.">
      <ThemeManager />
    </UiCard>
  </div>
</template>
