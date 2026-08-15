<script setup lang="ts">
import QRCode from 'qrcode'

const props = defineProps<{
  url: string
  label?: string
}>()

const open = defineModel<boolean>({ default: false })
const toast = useToast()
const dataUrl = ref('')
const generating = ref(false)

watch([open, () => props.url], async ([isOpen]) => {
  if (!isOpen || !props.url) return
  generating.value = true
  try {
    dataUrl.value = await QRCode.toDataURL(props.url, {
      errorCorrectionLevel: 'M',
      margin: 2,
      width: 640,
      color: { dark: '#0f172a', light: '#ffffff' },
    })
  } catch {
    dataUrl.value = ''
    toast.error('Could not generate the QR code')
  } finally {
    generating.value = false
  }
}, { immediate: true })

const filename = computed(() => {
  try {
    const parsed = new URL(props.url)
    return `qr-${parsed.hostname}-${parsed.pathname.replace(/^\/+|\/+$/g, '').replace(/[^a-z0-9_-]+/gi, '-') || 'shorturl'}.png`
  } catch {
    return 'shorturl-qr.png'
  }
})

async function copyImage() {
  if (!dataUrl.value || !navigator.clipboard || typeof ClipboardItem === 'undefined') {
    toast.error('Image copying is not supported by this browser. You can download the PNG instead.')
    return
  }
  try {
    const blob = await fetch(dataUrl.value).then(response => response.blob())
    await navigator.clipboard.write([new ClipboardItem({ 'image/png': blob })])
    toast.success('QR code copied as an image')
  } catch {
    toast.error('Could not copy the QR image. You can download the PNG instead.')
  }
}

function download() {
  if (!dataUrl.value) return
  const anchor = document.createElement('a')
  anchor.href = dataUrl.value
  anchor.download = filename.value
  anchor.click()
}
</script>

<template>
  <UiModal v-model="open" title="Short URL QR code" description="Scan to open the short URL, or save the image for print and sharing.">
    <div class="flex flex-col items-center gap-4">
      <div class="grid aspect-square w-full max-w-72 place-items-center overflow-hidden rounded-2xl border border-(--color-border) bg-white p-3 shadow-sm">
        <Icon v-if="generating" name="lucide:loader-circle" size="28" class="animate-spin text-slate-400" />
        <img v-else-if="dataUrl" :src="dataUrl" :alt="`QR code for ${label || url}`" class="size-full" />
      </div>
      <div class="w-full rounded-lg bg-(--color-surface-muted) px-3 py-2 text-center">
        <p v-if="label" class="truncate text-sm font-semibold">{{ label }}</p>
        <p class="truncate font-mono text-xs text-(--color-content-muted)">{{ url }}</p>
      </div>
    </div>

    <template #actions>
      <UiButton variant="secondary" @click="open = false">Close</UiButton>
      <UiButton variant="secondary" :disabled="!dataUrl" @click="download">
        <Icon name="lucide:download" size="16" /> Download PNG
      </UiButton>
      <UiButton :disabled="!dataUrl" @click="copyImage">
        <Icon name="lucide:copy" size="16" /> Copy QR
      </UiButton>
    </template>
  </UiModal>
</template>
