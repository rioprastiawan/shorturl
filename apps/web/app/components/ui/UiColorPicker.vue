<script setup lang="ts">
const props = withDefaults(defineProps<{ label?: string, hint?: string, presets?: string[], disabled?: boolean }>(), { label: undefined, hint: undefined, presets: () => ['#172033', '#16a34a', '#2563eb', '#7c3aed', '#dc2626', '#ffffff', '#000000'], disabled: false })
const model = defineModel<string>({ required: true })
const id = useId()
const open = ref(false)
const trigger = useTemplateRef<HTMLElement>('trigger')
const panel = useTemplateRef<HTMLElement>('panel')
const saturation = useTemplateRef<HTMLElement>('saturation')
const hueTrack = useTemplateRef<HTMLElement>('hueTrack')
const hue = ref(0)
const sat = ref(0)
const value = ref(0)
const valid = computed(() => /^#[0-9a-f]{6}$/i.test(model.value))
const displayColor = computed(() => valid.value ? model.value : '#000000')
const hueColor = computed(() => hsvToHex(hue.value, 100, 100))
const { floatingStyle } = useFloatingPanel(trigger, open, { width: 280, estimatedHeight: 360, flipThreshold: 160 })

watch(() => model.value, color => {
  if (!/^#[0-9a-f]{6}$/i.test(color)) return
  const next = hexToHsv(color)
  hue.value = next.h
  sat.value = next.s
  value.value = next.v
}, { immediate: true })

watch(open, async isOpen => {
  if (!isOpen) return
  await nextTick()
  if (panel.value && 'showPopover' in panel.value && !panel.value.matches(':popover-open')) panel.value.showPopover()
})

onClickOutside([trigger, panel], () => (open.value = false))

function setFromHsv() { model.value = hsvToHex(hue.value, sat.value, value.value) }
function startSaturation(event: PointerEvent) {
  if (props.disabled) return
  updateSaturation(event)
  window.addEventListener('pointermove', updateSaturation)
  window.addEventListener('pointerup', stopSaturation, { once: true })
}
function updateSaturation(event: PointerEvent) {
  const rect = saturation.value?.getBoundingClientRect()
  if (!rect) return
  sat.value = Math.round(Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width)) * 100)
  value.value = Math.round((1 - Math.max(0, Math.min(1, (event.clientY - rect.top) / rect.height))) * 100)
  setFromHsv()
}
function stopSaturation() { window.removeEventListener('pointermove', updateSaturation) }
function startHue(event: PointerEvent) {
  if (props.disabled) return
  updateHue(event)
  window.addEventListener('pointermove', updateHue)
  window.addEventListener('pointerup', stopHue, { once: true })
}
function updateHue(event: PointerEvent) {
  const rect = hueTrack.value?.getBoundingClientRect()
  if (!rect) return
  hue.value = Math.round(Math.max(0, Math.min(1, (event.clientY - rect.top) / rect.height)) * 359)
  setFromHsv()
}
function stopHue() { window.removeEventListener('pointermove', updateHue) }
function inputHex(event: Event) { model.value = `#${(event.target as HTMLInputElement).value.replace(/[^0-9a-f]/gi, '').slice(0, 6)}` }
function normalize() { const color = model.value.trim(); if (/^[0-9a-f]{6}$/i.test(color)) model.value = `#${color.toLowerCase()}`; else if (valid.value) model.value = color.toLowerCase() }
function select(color: string) { model.value = color; open.value = false }
onBeforeUnmount(() => { stopSaturation(); stopHue() })

function hexToHsv(hex: string) {
  const red = Number.parseInt(hex.slice(1, 3), 16) / 255; const green = Number.parseInt(hex.slice(3, 5), 16) / 255; const blue = Number.parseInt(hex.slice(5, 7), 16) / 255
  const max = Math.max(red, green, blue); const min = Math.min(red, green, blue); const delta = max - min
  let h = 0
  if (delta) { if (max === red) h = 60 * (((green - blue) / delta) % 6); else if (max === green) h = 60 * ((blue - red) / delta + 2); else h = 60 * ((red - green) / delta + 4) }
  if (h < 0) h += 360
  return { h: Math.round(h), s: Math.round(max ? delta / max * 100 : 0), v: Math.round(max * 100) }
}

function hsvToHex(h: number, s: number, v: number) {
  const saturation = s / 100; const brightness = v / 100; const chroma = brightness * saturation; const segment = h / 60; const x = chroma * (1 - Math.abs(segment % 2 - 1)); const offset = brightness - chroma
  let rgb: [number, number, number]
  if (segment < 1) rgb = [chroma, x, 0]; else if (segment < 2) rgb = [x, chroma, 0]; else if (segment < 3) rgb = [0, chroma, x]; else if (segment < 4) rgb = [0, x, chroma]; else if (segment < 5) rgb = [x, 0, chroma]; else rgb = [chroma, 0, x]
  return `#${rgb.map(channel => Math.round((channel + offset) * 255).toString(16).padStart(2, '0')).join('')}`
}
</script>

<template>
  <div class="flex flex-col gap-1.5" :class="disabled ? 'opacity-55' : ''">
    <label v-if="label" :for="id" class="text-sm font-medium">{{ label }}</label>
    <div class="flex rounded-lg border bg-(--color-surface-raised) p-1 shadow-sm transition-colors focus-within:border-(--color-accent)" :class="valid ? 'border-(--color-border-strong)' : 'border-(--color-danger)'">
      <button ref="trigger" type="button" class="relative grid size-9 shrink-0 place-items-center overflow-hidden rounded-md border border-black/10 shadow-inner" :style="{ background: displayColor }" :disabled="disabled" :aria-expanded="open" aria-haspopup="dialog" title="Open color picker" @click="open = !open"><Icon name="lucide:pipette" size="14" class="mix-blend-difference text-white" /></button>
      <span class="grid place-items-center px-2 font-mono text-xs text-(--color-content-subtle)">#</span>
      <input :id="id" :value="model.replace(/^#/, '')" maxlength="6" spellcheck="false" class="min-w-0 flex-1 bg-transparent pr-2 font-mono text-sm uppercase outline-none" :disabled="disabled" @input="inputHex" @blur="normalize">
    </div>

    <Transition name="menu-down"><div v-if="open" ref="panel" popover="manual" :style="floatingStyle" role="dialog" :aria-label="`${label || 'Color'} picker`" class="m-0 w-70 rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-3 text-(--color-content) shadow-2xl">
      <div class="grid grid-cols-[1fr_1.25rem] gap-3">
        <div ref="saturation" class="relative h-48 cursor-crosshair touch-none overflow-hidden rounded-lg border border-black/10" :style="{ backgroundColor: hueColor }" aria-label="Saturation and brightness" @pointerdown.prevent="startSaturation"><div class="absolute inset-0 bg-gradient-to-r from-white to-transparent" /><div class="absolute inset-0 bg-gradient-to-t from-black to-transparent" /><span class="pointer-events-none absolute size-4 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_1px_4px_rgba(0,0,0,.7)]" :style="{ left: `${sat}%`, top: `${100 - value}%`, backgroundColor: displayColor }" /></div>
        <div ref="hueTrack" class="relative h-48 cursor-ns-resize touch-none rounded-full border border-black/10 bg-[linear-gradient(to_bottom,#f00,#ff0,#0f0,#0ff,#00f,#f0f,#f00)]" aria-label="Hue spectrum" @pointerdown.prevent="startHue"><span class="pointer-events-none absolute left-1/2 h-2.5 w-7 -translate-x-1/2 -translate-y-1/2 rounded-full border-2 border-white shadow-[0_1px_4px_rgba(0,0,0,.65)]" :style="{ top: `${hue / 359 * 100}%`, backgroundColor: hueColor }" /></div>
      </div>
      <div class="mt-3 flex items-center gap-3"><span class="size-8 shrink-0 rounded-md border border-(--color-border) shadow-inner" :style="{ backgroundColor: displayColor }" /><div class="min-w-0"><p class="font-mono text-xs font-semibold uppercase">{{ displayColor }}</p><p class="text-[10px] text-(--color-content-subtle)">H {{ hue }}° · S {{ sat }}% · B {{ value }}%</p></div></div>
      <div class="mt-3 flex items-center rounded-lg border border-(--color-border-strong) bg-(--color-surface-muted) px-2.5"><span class="font-mono text-xs text-(--color-content-subtle)">HEX</span><input :value="model" maxlength="7" class="min-w-0 flex-1 bg-transparent px-2 py-2 text-right font-mono text-sm uppercase outline-none" @input="inputHex" @blur="normalize"></div>
      <div class="mt-3 grid grid-cols-7 gap-2"><button v-for="color in presets" :key="color" type="button" class="aspect-square rounded-full border border-black/10 shadow-sm transition-transform hover:scale-110" :class="model.toLowerCase() === color.toLowerCase() ? 'ring-2 ring-(--color-accent) ring-offset-2 ring-offset-(--color-surface-raised)' : ''" :style="{ backgroundColor: color }" :aria-label="`Use ${color}`" @click="select(color)" /></div>
    </div></Transition>

    <p v-if="!valid" class="text-xs text-(--color-danger)">Enter a complete six-digit hex color.</p><p v-else-if="hint" class="text-xs text-(--color-content-muted)">{{ hint }}</p>
  </div>
</template>
