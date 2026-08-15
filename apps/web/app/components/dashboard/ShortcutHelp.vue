<script setup lang="ts">
const open = defineModel<boolean>({ default: false })

const shortcuts = [
  { keys: ['C'], label: 'Create short link' },
  { keys: ['⌘/Ctrl', 'K'], label: 'Open quick command' },
  { keys: ['/'], label: 'Search links' },
  { keys: ['W'], label: 'Switch workspace' },
  { keys: ['G', 'O'], label: 'Go to overview' },
  { keys: ['G', 'L'], label: 'Go to links' },
  { keys: ['G', 'A'], label: 'Go to analytics' },
  { keys: ['G', 'D'], label: 'Go to domains' },
  { keys: ['G', 'M'], label: 'Go to members' },
  { keys: ['G', 'I'], label: 'Go to API integrations' },
  { keys: ['G', 'S'], label: 'Go to settings' },
  { keys: ['G', 'P'], label: 'Go to profile' },
  { keys: ['?'], label: 'Show keyboard shortcuts' },
]
</script>

<template>
  <UiModal v-model="open" title="Keyboard shortcuts" description="Move around ShortURL without leaving the keyboard." size="lg">
    <div class="grid gap-x-8 sm:grid-cols-2">
      <div v-for="shortcut in shortcuts" :key="shortcut.label" class="flex items-center justify-between gap-4 border-b border-(--color-border) py-3">
        <span class="text-sm text-(--color-content-muted)">{{ shortcut.label }}</span>
        <span class="flex shrink-0 items-center gap-1">
          <kbd v-for="key in shortcut.keys" :key="key" class="min-w-7 rounded-md border border-(--color-border-strong) bg-(--color-surface-muted) px-2 py-1 text-center font-sans text-xs font-semibold shadow-sm">{{ key }}</kbd>
        </span>
      </div>
    </div>
    <template #actions>
      <UiButton variant="secondary" @click="open = false">Close</UiButton>
    </template>
  </UiModal>
</template>
