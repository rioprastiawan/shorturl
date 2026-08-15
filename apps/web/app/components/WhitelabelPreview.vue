<script setup lang="ts">
import type { SystemBranding } from '~/types/api'

const props = defineProps<{ value: SystemBranding }>()
const view = ref<'login' | 'dashboard' | 'mobile'>('login')
const logo = computed(() => props.value.logo_dark_url || props.value.logo_compact_url || props.value.logo_light_url)
const foreground = computed(() => contrastColor(props.value.primary_color))

function contrastColor(hex: string) {
  if (!/^#[0-9a-f]{6}$/i.test(hex)) return '#ffffff'
  const red = Number.parseInt(hex.slice(1, 3), 16)
  const green = Number.parseInt(hex.slice(3, 5), 16)
  const blue = Number.parseInt(hex.slice(5, 7), 16)
  return (0.299 * red + 0.587 * green + 0.114 * blue) / 255 > 0.6 ? '#172033' : '#ffffff'
}
</script>

<template>
  <UiCard title="Live preview" description="Preview unsaved identity, navigation, and authentication changes.">
    <div class="mb-3 flex flex-wrap gap-1 rounded-lg bg-(--color-surface-muted) p-1">
      <button v-for="option in (['login', 'dashboard', 'mobile'] as const)" :key="option" type="button" class="rounded-md px-3 py-1.5 text-xs font-semibold capitalize transition-colors" :class="view === option ? 'bg-(--color-surface-raised) text-(--color-content) shadow-sm' : 'text-(--color-content-muted)'" @click="view = option">{{ option }}</button>
    </div>

    <div class="overflow-hidden rounded-xl border border-(--color-border) bg-white shadow-sm">
      <div class="flex h-7 items-center gap-1.5 border-b border-slate-200 bg-slate-100 px-3"><span v-for="dot in 3" :key="dot" class="size-2 rounded-full bg-slate-300" /><span class="ml-2 h-3 flex-1 rounded bg-white" /></div>

      <div v-if="view === 'login'" class="grid min-h-72 grid-cols-[1.05fr_.95fr]">
        <section class="flex flex-col justify-between p-5 text-white" :style="{ backgroundColor: value.shell_color }">
          <div class="flex items-center gap-2 text-sm font-bold"><img v-if="logo" :src="logo" alt="" class="h-7 max-w-28 object-contain"><template v-else><span class="grid size-7 place-items-center rounded-md" :style="{ backgroundColor: value.primary_color, color: foreground }"><Icon name="lucide:link-2" size="14" /></span><span>{{ value.app_name || 'Application' }}</span></template></div>
          <div><p class="text-[9px] font-bold uppercase tracking-widest" :style="{ color: value.primary_color }">{{ value.tagline }}</p><p class="mt-2 text-lg font-bold leading-tight">{{ value.login_heading }}</p><p class="mt-2 line-clamp-3 text-[10px] leading-relaxed opacity-65">{{ value.login_description }}</p></div>
          <p class="text-[8px] opacity-50">{{ value.footer_text }}</p>
        </section>
        <section class="grid place-items-center bg-white p-5 text-slate-900"><div class="w-full max-w-40"><p class="text-base font-bold">Welcome back</p><p class="mb-4 text-[9px] text-slate-500">Sign in to continue.</p><div class="mb-2 h-7 rounded border border-slate-200" /><div class="mb-3 h-7 rounded border border-slate-200" /><div class="grid h-7 place-items-center rounded text-[9px] font-bold" :style="{ backgroundColor: value.primary_color, color: foreground }">Sign in</div></div></section>
      </div>

      <div v-else-if="view === 'dashboard'" class="grid min-h-72 grid-cols-[8rem_1fr] bg-slate-50">
        <aside class="flex flex-col p-3 text-white" :style="{ backgroundColor: value.shell_color }"><div class="flex items-center gap-2 text-xs font-bold"><img v-if="logo" :src="logo" alt="" class="h-6 max-w-20 object-contain"><span v-else>{{ value.app_name || 'Application' }}</span></div><div class="mt-6 space-y-1"><div v-for="(item, index) in ['Overview', 'Links', 'Analytics', 'Domains']" :key="item" class="flex items-center gap-2 rounded px-2 py-1.5 text-[9px]" :style="index === 0 ? { color: value.primary_color, backgroundColor: 'rgb(255 255 255 / .07)' } : { opacity: '.6' }"><span class="size-2 rounded-sm border" />{{ item }}</div></div></aside>
        <main class="p-4 text-slate-900"><p class="text-base font-bold">Overview</p><p class="text-[9px] text-slate-500">Your link performance at a glance.</p><div class="mt-4 grid grid-cols-3 gap-2"><div v-for="card in 3" :key="card" class="h-14 rounded-md border border-slate-200 bg-white p-2"><div class="h-2 w-1/2 rounded bg-slate-100" /><div class="mt-2 h-3 w-1/3 rounded" :style="{ backgroundColor: value.primary_color }" /></div></div><div class="mt-3 h-28 rounded-md border border-slate-200 bg-white p-3"><div class="h-full rounded opacity-15" :style="{ background: `linear-gradient(135deg, transparent 30%, ${value.primary_color} 30% 34%, transparent 34% 55%, ${value.primary_color} 55% 59%, transparent 59%)` }" /></div></main>
      </div>

      <div v-else class="mx-auto min-h-72 max-w-56 bg-slate-50 text-slate-900"><header class="flex h-11 items-center justify-between px-3 text-white" :style="{ backgroundColor: value.shell_color }"><span class="text-xs font-bold">{{ value.app_name || 'Application' }}</span><span class="size-6 rounded-full" :style="{ backgroundColor: value.primary_color }" /></header><main class="p-3"><p class="text-sm font-bold">Overview</p><div class="mt-3 space-y-2"><div v-for="card in 3" :key="card" class="h-12 rounded-md border border-slate-200 bg-white" /></div></main><nav class="mt-2 grid h-11 grid-cols-4 place-items-center border-t border-slate-200 text-[8px]" :style="{ color: value.primary_color }"><span>Home</span><span>Links</span><span class="grid size-7 place-items-center rounded-lg" :style="{ backgroundColor: value.primary_color, color: foreground }">+</span><span>More</span></nav></div>
    </div>
    <p class="mt-2 text-xs text-(--color-content-subtle)">Preview only. Click “Save whitelabeling” to apply these changes globally.</p>
  </UiCard>
</template>
