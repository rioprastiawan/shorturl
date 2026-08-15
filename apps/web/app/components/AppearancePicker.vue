<script setup lang="ts">
withDefaults(defineProps<{ compact?: boolean }>(), { compact: false })

const colorMode = useColorMode()
const { t } = useUserPreferences()
const options = computed(() => [
  { value: 'light', label: t('light'), icon: 'lucide:sun' },
  { value: 'dark', label: t('dark'), icon: 'lucide:moon' },
  { value: 'system', label: t('systemMode'), icon: 'lucide:monitor' },
])
</script>

<template>
  <ColorScheme placeholder="" tag="div">
    <div
      class="grid grid-cols-3 gap-1 rounded-xl p-1"
      :class="compact ? 'bg-white/7' : 'border border-(--color-border) bg-(--color-surface-muted)'"
      aria-label="Appearance"
    >
      <button
        v-for="option in options"
        :key="option.value"
        type="button"
        class="flex flex-col items-center justify-center gap-1 rounded-lg px-2 py-2 text-xs font-semibold transition-all"
        :class="colorMode.preference === option.value
          ? compact
            ? 'bg-white text-[#172033] shadow-sm dark:bg-[#293341] dark:text-white'
            : 'bg-(--color-surface-raised) text-(--color-content) shadow-sm ring-1 ring-(--color-border)'
          : compact
            ? 'text-white/55 hover:bg-white/8 hover:text-white'
            : 'text-(--color-content-muted) hover:text-(--color-content)'"
        :aria-pressed="colorMode.preference === option.value"
        :title="option.label"
        @click="colorMode.preference = option.value"
      >
        <Icon :name="option.icon" size="18" class="shrink-0" />
        <span v-if="!compact">{{ option.label }}</span>
        <span v-else class="sr-only">{{ option.label }}</span>
      </button>
    </div>
  </ColorScheme>
</template>
