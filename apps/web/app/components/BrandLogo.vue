<script setup lang="ts">
const props = withDefaults(defineProps<{ compact?: boolean, inverse?: boolean }>(), { compact: false, inverse: false })
const { branding } = useBranding()
const colorMode = useColorMode()
const logoUrl = computed(() => {
  if (props.compact && branding.value.logo_compact_url) return branding.value.logo_compact_url
  if (props.inverse) return branding.value.logo_dark_url || branding.value.logo_light_url
  return colorMode.value === 'dark'
    ? (branding.value.logo_dark_url || branding.value.logo_light_url)
    : (branding.value.logo_light_url || branding.value.logo_dark_url)
})
</script>

<template>
  <span class="inline-flex min-w-0 items-center gap-2.5">
    <img v-if="logoUrl" :src="logoUrl" :alt="`${branding.app_name} logo`" class="shrink-0 object-contain" :class="compact ? 'size-8' : 'h-10 max-w-48'">
    <span v-else class="grid shrink-0 place-items-center rounded-lg bg-(--color-shell-accent) text-(--color-accent-content)" :class="compact ? 'size-8' : 'size-10'">
      <Icon name="lucide:link-2" :size="compact ? 17 : 20" />
    </span>
    <span v-if="!compact" class="truncate font-bold tracking-tight" :class="inverse ? 'text-(--color-shell-content)' : 'text-(--color-content)'">{{ branding.app_name }}</span>
  </span>
</template>
