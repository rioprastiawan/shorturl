<script setup lang="ts">
const props = withDefaults(defineProps<{
  value: string
  label?: string
  /** Renders the value alongside the button in a monospace field. */
  showValue?: boolean
}>(), {
  label: 'Copy',
  showValue: false,
})

const toast = useToast()
const copied = ref(false)

// One click, per plan §38. The clipboard API needs a secure context, so fall
// back to a hidden textarea when the dashboard is served over plain http.
async function copy() {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(props.value)
    } else {
      legacyCopy(props.value)
    }
    copied.value = true
    toast.success('Copied to clipboard')
    setTimeout(() => (copied.value = false), 1500)
  } catch {
    toast.error('Could not copy to clipboard')
  }
}

function legacyCopy(text: string) {
  const el = document.createElement('textarea')
  el.value = text
  el.setAttribute('readonly', '')
  el.style.position = 'fixed'
  el.style.opacity = '0'
  document.body.appendChild(el)
  el.select()
  document.execCommand('copy')
  document.body.removeChild(el)
}
</script>

<template>
  <div v-if="showValue" class="flex items-stretch">
    <code
      class="min-w-0 flex-1 truncate rounded-l-md border border-r-0 border-(--color-border-strong) bg-(--color-surface-muted) px-3 py-2 font-mono text-xs"
    >{{ value }}</code>
    <button
      class="shrink-0 rounded-r-md border border-(--color-border-strong) px-3 text-xs font-medium transition-colors hover:bg-(--color-surface-muted)"
      type="button"
      @click="copy"
    >
      {{ copied ? 'Copied' : label }}
    </button>
  </div>

  <button
    v-else
    type="button"
    class="inline-flex items-center gap-1.5 rounded-md px-2 py-1 text-xs font-medium text-(--color-content-muted) transition-colors hover:bg-(--color-surface-muted) hover:text-(--color-content)"
    @click="copy"
  >
    {{ copied ? 'Copied' : label }}
  </button>
</template>
