<script setup lang="ts">
import type { CustomTheme, ThemePalette } from '~/composables/useCustomThemes'
import { DEFAULT_DARK_PALETTE, DEFAULT_LIGHT_PALETTE, THEME_TOKENS } from '~/composables/useCustomThemes'

const { themes, activeId, save, activate, remove } = useCustomThemes()
const toast = useToast()
const editor = ref<CustomTheme | null>(null)
const paletteMode = ref<'light' | 'dark'>('light')

function copyPalette(palette: ThemePalette): ThemePalette {
  return { ...palette }
}

function createTheme() {
  editor.value = {
    id: globalThis.crypto?.randomUUID?.() ?? `theme-${Date.now()}`,
    name: 'My custom theme',
    light: copyPalette(DEFAULT_LIGHT_PALETTE),
    dark: copyPalette(DEFAULT_DARK_PALETTE),
    updatedAt: new Date().toISOString(),
  }
  paletteMode.value = 'light'
}

function editTheme(theme: CustomTheme) {
  editor.value = {
    ...theme,
    light: copyPalette(theme.light),
    dark: copyPalette(theme.dark),
  }
  paletteMode.value = 'light'
}

function saveTheme() {
  if (!editor.value) return
  const name = editor.value.name.trim()
  if (!name) {
    toast.error('Give the custom theme a name.')
    return
  }
  save({ ...editor.value, name: name.slice(0, 40), updatedAt: new Date().toISOString() })
  editor.value = null
  toast.success('Custom theme saved and applied')
}

function deleteTheme(theme: CustomTheme) {
  remove(theme.id)
  if (editor.value?.id === theme.id) editor.value = null
  toast.success(`Deleted ${theme.name}`)
}
</script>

<template>
  <div class="space-y-4">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <p class="text-sm font-semibold">Custom themes</p>
        <p class="mt-0.5 text-xs text-(--color-content-muted)">Each theme contains a Light and Dark palette and stays in this browser.</p>
      </div>
      <UiButton size="sm" @click="createTheme"><Icon name="lucide:palette" size="15" /> New custom theme</UiButton>
    </div>

    <div v-if="themes.length" class="grid gap-2 sm:grid-cols-2">
      <div
        v-for="theme in themes"
        :key="theme.id"
        class="rounded-lg border p-3"
        :class="activeId === theme.id ? 'border-(--color-accent) bg-(--color-accent)/5' : 'border-(--color-border)'"
      >
        <div class="flex items-center justify-between gap-2">
          <div class="min-w-0">
            <p class="truncate text-sm font-semibold">{{ theme.name }}</p>
            <p class="text-xs text-(--color-content-muted)">{{ activeId === theme.id ? 'Applied' : 'Custom' }}</p>
          </div>
          <div class="flex gap-1">
            <span v-for="color in [theme.light.surface, theme.light.accent, theme.dark.surface, theme.dark.accent]" :key="color" class="size-5 rounded-full border border-black/10" :style="{ backgroundColor: color }" />
          </div>
        </div>
        <div class="mt-3 flex flex-wrap gap-2">
          <UiButton v-if="activeId !== theme.id" variant="secondary" size="sm" @click="activate(theme.id)">Apply</UiButton>
          <UiButton v-else variant="secondary" size="sm" @click="activate(null)">Use default</UiButton>
          <UiButton variant="ghost" size="sm" @click="editTheme(theme)">Edit</UiButton>
          <UiButton variant="ghost" size="sm" @click="deleteTheme(theme)"><span class="text-(--color-danger)">Delete</span></UiButton>
        </div>
      </div>
    </div>

    <div v-if="editor" class="rounded-xl border border-(--color-border) bg-(--color-surface-muted)/45 p-4">
      <div class="flex flex-wrap items-end gap-3">
        <div class="min-w-52 flex-1"><UiInput v-model="editor.name" label="Theme name" maxlength="40" /></div>
        <div class="grid grid-cols-2 gap-1 rounded-lg border border-(--color-border) bg-(--color-surface-raised) p-1">
          <button v-for="mode in (['light', 'dark'] as const)" :key="mode" type="button" class="rounded-md px-3 py-1.5 text-xs font-semibold capitalize" :class="paletteMode === mode ? 'bg-(--color-accent) text-(--color-accent-content)' : 'text-(--color-content-muted)'" @click="paletteMode = mode">
            <Icon :name="mode === 'light' ? 'lucide:sun' : 'lucide:moon'" size="14" class="mr-1 inline" />{{ mode }}
          </button>
        </div>
      </div>

      <div class="mt-4 grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
        <label v-for="token in THEME_TOKENS" :key="token.key" class="flex items-center gap-2 rounded-lg border border-(--color-border) bg-(--color-surface-raised) p-2">
          <input v-model="editor[paletteMode][token.key]" type="color" class="size-8 shrink-0 cursor-pointer rounded border-0 bg-transparent p-0" :aria-label="token.label">
          <span class="min-w-0 flex-1"><span class="block truncate text-xs font-medium">{{ token.label }}</span><span class="block font-mono text-[10px] text-(--color-content-subtle)">{{ editor[paletteMode][token.key] }}</span></span>
        </label>
      </div>

      <div class="mt-4 flex justify-end gap-2">
        <UiButton variant="secondary" @click="editor = null">Cancel</UiButton>
        <UiButton @click="saveTheme">Save & apply</UiButton>
      </div>
    </div>
  </div>
</template>
