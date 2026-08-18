<script setup lang="ts">
import QRCode from 'qrcode'
import type { SystemBranding } from '~/types/api'
import { ApiError } from '~/composables/useApi'
import { DEFAULT_BRANDING } from '~/composables/useBranding'
import { drawQrFinders, drawQrModule, isFinderModule } from '~/utils/qrRenderer'

definePageMeta({ middleware: ['auth', 'workspace-admin'] })
useHead({ title: 'QR Branding · ShortURL' })

const session = useSession()
const { load, set } = useBranding()
const { branding: service } = useServices()
const toast = useToast()
const loading = ref(true)
const saving = ref(false)
const preview = ref('')
const form = reactive<SystemBranding>({ ...DEFAULT_BRANDING })
const isAdmin = computed(() => Boolean(session.user.value?.is_admin))
const styleOptions = [
  { label: 'Square', value: 'square' }, { label: 'Rounded', value: 'rounded' }, { label: 'Dots', value: 'dots' },
  { label: 'Extra rounded', value: 'extra-rounded' }, { label: 'Diamond', value: 'diamond' }, { label: 'Classy', value: 'classy' },
  { label: 'Classy rounded', value: 'classy-rounded' }, { label: 'Soft', value: 'soft' }, { label: 'Star', value: 'star' },
]
const cornerOptions = [{ label: 'Square', value: 'square' }, { label: 'Rounded', value: 'rounded' }, { label: 'Circle', value: 'circle' }, { label: 'Dot', value: 'dot' }, { label: 'Leaf', value: 'leaf' }]
const sizeOptions = [{ label: '512 px', value: 512 }, { label: '1024 px', value: 1024 }, { label: '2048 px', value: 2048 }]
let previewGeneration = 0

onMounted(async () => {
  if (isAdmin.value) Object.assign(form, await load(true))
  loading.value = false
})

watch(() => [form.qr_foreground, form.qr_background, form.qr_style, form.qr_corner_style, form.qr_margin, form.qr_size, form.qr_use_logo, form.logo_compact_url, form.logo_light_url], async () => {
  if (!import.meta.client) return
  const generation = ++previewGeneration
  try {
    if (!/^#[0-9a-f]{6}$/i.test(form.qr_foreground) || !/^#[0-9a-f]{6}$/i.test(form.qr_background)) return
    const logo = form.qr_use_logo ? (form.logo_compact_url || form.logo_light_url) : ''
    const qr = QRCode.create('https://example.com/preview', { errorCorrectionLevel: logo ? 'H' : 'M' })
    const count = qr.modules.size; const total = count + form.qr_margin * 2; const size = 640; const cell = size / total
    const canvas = document.createElement('canvas'); canvas.width = size; canvas.height = size
    const context = canvas.getContext('2d'); if (!context) return
    context.fillStyle = form.qr_background; context.fillRect(0, 0, size, size); context.fillStyle = form.qr_foreground
    for (let row = 0; row < count; row++) for (let column = 0; column < count; column++) {
      if (!qr.modules.get(row, column)) continue
      if (isFinderModule(row, column, count)) continue
      drawQrModule(context, (column + form.qr_margin) * cell, (row + form.qr_margin) * cell, cell, form.qr_style, row, column)
    }
    drawQrFinders(context, count, form.qr_margin, cell, form.qr_corner_style, form.qr_foreground, form.qr_background)
    if (logo) {
      try {
        const image = await new Promise<HTMLImageElement>((resolve, reject) => { const item = new Image(); item.crossOrigin = 'anonymous'; item.onload = () => resolve(item); item.onerror = reject; item.src = logo })
        const box = size * 0.22; const pad = size * 0.018; const x = (size - box) / 2
        context.fillStyle = form.qr_background; context.beginPath(); context.roundRect(x - pad, x - pad, box + pad * 2, box + pad * 2, size * 0.025); context.fill()
        const ratio = Math.min(box / image.width, box / image.height); const width = image.width * ratio; const height = image.height * ratio
        context.drawImage(image, (size - width) / 2, (size - height) / 2, width, height)
      } catch { /* Keep the QR preview usable if the logo cannot load. */ }
    }
    if (generation === previewGeneration) preview.value = canvas.toDataURL('image/png')
  } catch { /* Keep the previous preview while a hex color is incomplete. */ }
}, { immediate: true })

function preset(name: 'brand' | 'classic' | 'inverse') {
  if (name === 'brand') Object.assign(form, { qr_foreground: form.primary_color, qr_background: '#ffffff', qr_style: 'rounded', qr_corner_style: 'rounded', qr_margin: 2, qr_use_logo: true })
  if (name === 'classic') Object.assign(form, { qr_foreground: '#000000', qr_background: '#ffffff', qr_style: 'square', qr_corner_style: 'square', qr_margin: 4, qr_use_logo: false })
  if (name === 'inverse') Object.assign(form, { qr_foreground: '#ffffff', qr_background: form.shell_color, qr_style: 'dots', qr_corner_style: 'circle', qr_margin: 3, qr_use_logo: true })
}

async function save() {
  saving.value = true
  try {
    const updated = await service.update({ ...form })
    Object.assign(form, updated); set(updated)
    toast.success('QR branding saved')
  } catch (error) { toast.error(error instanceof ApiError ? error.message : 'Could not save QR branding.') }
  finally { saving.value = false }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header><p class="mb-1 text-sm font-semibold text-(--color-accent)">System</p><h1 class="text-2xl font-bold tracking-tight sm:text-3xl">QR Branding</h1><p class="text-sm text-(--color-content-muted)">Customize the default appearance and brand identity of every QR code.</p></header>
    <UiCard v-if="!isAdmin" :padded="false"><UiEmptyState title="System administrator access required" description="Only installation administrators can change global QR branding." /></UiCard>
    <div v-else-if="loading" class="grid gap-4 lg:grid-cols-[20rem_1fr]"><UiSkeleton height="24rem" rounded="lg" /><UiSkeleton height="24rem" rounded="lg" /></div>
    <form v-else class="grid items-start gap-5 lg:grid-cols-[20rem_1fr]" @submit.prevent="save">
      <UiCard title="QR preview" description="This is the same renderer used by every link QR modal."><div class="grid aspect-square place-items-center overflow-hidden rounded-xl border border-(--color-border) p-3" :style="{ backgroundColor: form.qr_background }"><img v-if="preview" :src="preview" alt="QR branding preview" class="size-full"></div><p class="mt-3 text-xs text-(--color-content-subtle)">Changes are previewed instantly and apply globally after saving.</p></UiCard>
      <div class="flex flex-col gap-5">
        <UiCard title="QR defaults" description="Changes apply globally after saving.">
          <div class="mb-4 flex flex-wrap gap-2"><UiButton type="button" size="sm" variant="secondary" @click="preset('brand')">Brand</UiButton><UiButton type="button" size="sm" variant="secondary" @click="preset('classic')">Classic</UiButton><UiButton type="button" size="sm" variant="secondary" @click="preset('inverse')">Inverse</UiButton></div>
          <div class="grid gap-4 sm:grid-cols-2"><UiColorPicker v-model="form.qr_foreground" label="Foreground" hint="Keep strong contrast with the background." /><UiColorPicker v-model="form.qr_background" label="Background" /></div>
          <div class="mt-4 grid gap-4 sm:grid-cols-2"><UiSelect v-model="form.qr_style" label="Module style" :options="styleOptions" /><UiSelect v-model="form.qr_corner_style" label="Corner style" :options="cornerOptions" /><UiSelect v-model="form.qr_size" label="PNG export size" :options="sizeOptions" /></div>
          <div class="mt-5"><UiSlider v-model="form.qr_margin" label="Quiet zone" :min="1" :max="6" suffix=" modules" hint="A larger clear area makes QR codes easier to scan in print." /></div>
          <div class="mt-4"><UiCheckbox v-model="form.qr_use_logo" :disabled="!form.logo_compact_url && !form.logo_light_url">Place the Whitelabeling logo in the center</UiCheckbox><p v-if="!form.logo_compact_url && !form.logo_light_url" class="mt-1 text-xs text-(--color-content-subtle)">Upload a compact or light logo in Whitelabeling to enable this option.</p></div>
        </UiCard>
        <div class="flex justify-end"><UiButton class="w-full sm:w-auto" type="submit" :loading="saving">Save QR branding</UiButton></div>
      </div>
    </form>
  </div>
</template>
