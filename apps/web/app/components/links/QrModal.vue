<script setup lang="ts">
import QRCode from 'qrcode'
import { drawQrFinders, drawQrModule, isFinderModule, qrSvgFinders, qrSvgModule } from '~/utils/qrRenderer'

const props = defineProps<{ url: string, label?: string }>()
const open = defineModel<boolean>({ default: false })
const toast = useToast()
const { branding } = useBranding()
const dataUrl = ref('')
const svg = ref('')
const generating = ref(false)
const logo = computed(() => branding.value.logo_compact_url || branding.value.logo_light_url)

watch([open, () => props.url, branding], ([isOpen]) => {
  if (isOpen && props.url) generate()
}, { deep: true, immediate: true })

async function generate() {
  generating.value = true
  try {
    const useLogo = branding.value.qr_use_logo && Boolean(logo.value)
    const qr = QRCode.create(props.url, { errorCorrectionLevel: useLogo ? 'H' : 'M' })
    const count = qr.modules.size
    const margin = branding.value.qr_margin
    const size = branding.value.qr_size
    const total = count + margin * 2
    const canvas = document.createElement('canvas'); canvas.width = size; canvas.height = size
    const context = canvas.getContext('2d'); if (!context) throw new Error()
    const foreground = safeColor(branding.value.qr_foreground, '#172033')
    const background = safeColor(branding.value.qr_background, '#ffffff')
    context.fillStyle = background; context.fillRect(0, 0, size, size); context.fillStyle = foreground
    const cell = size / total
    for (let row = 0; row < count; row++) for (let column = 0; column < count; column++) {
      if (qr.modules.get(row, column) && !isFinderModule(row, column, count)) drawQrModule(context, (column + margin) * cell, (row + margin) * cell, cell, branding.value.qr_style, row, column)
    }
    drawQrFinders(context, count, margin, cell, branding.value.qr_corner_style, foreground, background)
    let effectiveLogo = useLogo ? logo.value : ''
    if (effectiveLogo) { try { await drawLogo(context, effectiveLogo, size, background) } catch { effectiveLogo = '' } }
    dataUrl.value = canvas.toDataURL('image/png')
    svg.value = await createSvg(qr.modules, count, effectiveLogo, foreground, background)
  } catch { dataUrl.value = ''; svg.value = ''; toast.error('Could not generate the QR code') }
  finally { generating.value = false }
}

async function drawLogo(context: CanvasRenderingContext2D, source: string, size: number, background: string) {
  const image = await new Promise<HTMLImageElement>((resolve, reject) => { const item = new Image(); item.crossOrigin = 'anonymous'; item.onload = () => resolve(item); item.onerror = reject; item.src = source })
  const box = size * 0.22; const pad = size * 0.018; const x = (size - box) / 2
  context.fillStyle = background; context.beginPath(); context.roundRect(x - pad, x - pad, box + pad * 2, box + pad * 2, size * 0.025); context.fill()
  const ratio = Math.min(box / image.width, box / image.height); const width = image.width * ratio; const height = image.height * ratio
  context.drawImage(image, (size - width) / 2, (size - height) / 2, width, height)
}

async function createSvg(modules: { get: (row: number, column: number) => number }, count: number, logoSource: string, foreground: string, background: string) {
  const margin = branding.value.qr_margin; const total = count + margin * 2; const style = branding.value.qr_style
  const pieces = [`<svg xmlns="http://www.w3.org/2000/svg" width="${branding.value.qr_size}" height="${branding.value.qr_size}" viewBox="0 0 ${total} ${total}">`, `<rect width="${total}" height="${total}" fill="${background}"/>`, `<g fill="${foreground}">`]
  for (let row = 0; row < count; row++) for (let column = 0; column < count; column++) {
    if (!modules.get(row, column) || isFinderModule(row, column, count)) continue
    const x = column + margin; const y = row + margin
    pieces.push(qrSvgModule(x, y, style, row, column))
  }
  pieces.push('</g>', qrSvgFinders(count, margin, branding.value.qr_corner_style, foreground, background))
  if (logoSource) {
    const embedded = await embedImage(logoSource); const box = total * 0.22; const x = (total - box) / 2; const pad = total * 0.018
    pieces.push(`<rect x="${x - pad}" y="${x - pad}" width="${box + pad * 2}" height="${box + pad * 2}" rx="${total * 0.025}" fill="${background}"/>`, `<image href="${embedded}" x="${x}" y="${x}" width="${box}" height="${box}" preserveAspectRatio="xMidYMid meet"/>`)
  }
  return `${pieces.join('')}</svg>`
}

async function embedImage(source: string) { if (source.startsWith('data:')) return source; try { const blob = await fetch(source).then(response => response.blob()); return await new Promise<string>((resolve, reject) => { const reader = new FileReader(); reader.onload = () => resolve(String(reader.result)); reader.onerror = reject; reader.readAsDataURL(blob) }) } catch { return '' } }
function safeColor(value: string, fallback: string) { return /^#[0-9a-f]{6}$/i.test(value) ? value : fallback }
const filename = computed(() => { try { const parsed = new URL(props.url); return `qr-${parsed.hostname}-${parsed.pathname.replace(/^\/+|\/+$/g, '').replace(/[^a-z0-9_-]+/gi, '-') || 'shorturl'}` } catch { return 'shorturl-qr' } })
function download(href: string, extension: string) { const anchor = document.createElement('a'); anchor.href = href; anchor.download = `${filename.value}.${extension}`; anchor.click() }
function downloadSvg() { if (!svg.value) return; const url = URL.createObjectURL(new Blob([svg.value], { type: 'image/svg+xml' })); download(url, 'svg'); setTimeout(() => URL.revokeObjectURL(url), 1000) }
async function copyImage() { try { if (!dataUrl.value || typeof ClipboardItem === 'undefined') throw new Error(); const blob = await fetch(dataUrl.value).then(response => response.blob()); await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })]); toast.success('QR code copied as an image') } catch { toast.error('Image copying is unavailable. Download the PNG instead.') } }
function close() { open.value = false }
function onKeydown(event: KeyboardEvent) { if (event.key === 'Escape' && open.value) close() }
watch(open, value => { if (import.meta.client) document.body.style.overflow = value ? 'hidden' : '' })
onMounted(() => window.addEventListener('keydown', onKeydown))
onBeforeUnmount(() => { window.removeEventListener('keydown', onKeydown); document.body.style.overflow = '' })
</script>

<template>
  <Teleport to="body"><Transition name="backdrop-fade"><div v-if="open" class="fixed inset-0 z-[100] grid place-items-center overflow-y-auto bg-black/45 p-3 backdrop-blur-[2px]" @mousedown.self="close"><section role="dialog" aria-modal="true" aria-labelledby="qr-modal-title" class="my-auto w-full max-w-md rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-4 text-(--color-content) shadow-2xl">
    <div class="flex items-start justify-between gap-3"><div><h2 id="qr-modal-title" class="font-semibold">Short URL QR code</h2><p class="mt-1 text-sm text-(--color-content-muted)">Scan, copy, or download using the system appearance.</p></div><button type="button" class="rounded-md p-1.5 text-(--color-content-muted) hover:bg-(--color-surface-muted)" aria-label="Close" @click="close"><Icon name="lucide:x" size="18" /></button></div>
    <div class="mt-4 grid aspect-square place-items-center overflow-hidden rounded-2xl border border-(--color-border) bg-white p-3 shadow-sm"><div v-if="generating" class="grid size-full grid-cols-7 gap-1 p-2"><UiSkeleton v-for="cell in 49" :key="cell" height="100%" rounded="sm" /></div><img v-else-if="dataUrl" :src="dataUrl" :alt="`QR code for ${label || url}`" class="size-full" /></div>
    <div class="mt-3 rounded-lg bg-(--color-surface-muted) px-3 py-2 text-center"><p v-if="label" class="truncate text-sm font-semibold">{{ label }}</p><p class="truncate font-mono text-xs text-(--color-content-muted)">{{ url }}</p></div>
    <div class="mt-4 flex flex-wrap justify-end gap-2"><UiButton variant="secondary" @click="close">Close</UiButton><UiButton variant="secondary" :disabled="!svg" @click="downloadSvg"><Icon name="lucide:file-code-2" size="16" />SVG</UiButton><UiButton variant="secondary" :disabled="!dataUrl" @click="download(dataUrl, 'png')"><Icon name="lucide:download" size="16" />PNG</UiButton><UiButton :disabled="!dataUrl" @click="copyImage"><Icon name="lucide:copy" size="16" />Copy</UiButton></div>
  </section></div></Transition></Teleport>
</template>
