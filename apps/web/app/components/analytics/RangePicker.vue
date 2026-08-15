<script setup lang="ts">
import type { CustomRange, PresetRange } from './types'
import { RANGE_OPTIONS } from './types'

const props = defineProps<{
  disabled?: boolean
  customRange?: CustomRange | null
}>()
const emit = defineEmits<{ custom: [range: CustomRange] }>()
const model = defineModel<PresetRange>({ required: true })

const customOpen = ref(false)
const customTrigger = useTemplateRef<HTMLElement>('customTrigger')
const customPanel = useTemplateRef<HTMLElement>('customPanel')
const { floatingStyle } = useFloatingPanel(customTrigger, customOpen, {
  align: 'end', width: 480, estimatedHeight: 330, flipThreshold: 180,
})
const from = ref('')
const to = ref('')
const error = ref('')

function localValue(date: Date) {
  const pad = (value: number) => String(value).padStart(2, '0')
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function openCustom() {
  const now = new Date()
  const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000)
  from.value = props.customRange?.from ? localValue(new Date(props.customRange.from)) : localValue(weekAgo)
  to.value = props.customRange?.to ? localValue(new Date(props.customRange.to)) : localValue(now)
  error.value = ''
  customOpen.value = true
}

function applyCustom() {
  const start = new Date(from.value)
  const end = new Date(to.value)
  if (!from.value || !to.value || Number.isNaN(start.getTime()) || Number.isNaN(end.getTime())) {
    error.value = 'Choose a valid start and end date.'
    return
  }
  if (start >= end) {
    error.value = 'The start must be earlier than the end.'
    return
  }
  if (end.getTime() - start.getTime() > 366 * 24 * 60 * 60 * 1000) {
    error.value = 'Custom ranges may span at most 366 days.'
    return
  }
  emit('custom', { from: start.toISOString(), to: end.toISOString() })
  model.value = 'custom'
  customOpen.value = false
}

watch(customOpen, async (isOpen) => {
  if (!isOpen) return
  await nextTick()
  const element = customPanel.value
  if (element && 'showPopover' in element && !element.matches(':popover-open')) element.showPopover()
})
</script>

<template>
  <div class="relative">
    <div class="inline-flex flex-wrap rounded-md border border-(--color-border-strong) p-0.5" role="group" aria-label="Date range">
      <button
        v-for="option in RANGE_OPTIONS"
        :key="option.value"
        type="button"
        :disabled="disabled"
        class="rounded px-2.5 py-1 text-sm font-medium transition-colors disabled:opacity-50"
        :class="model === option.value ? 'bg-(--color-accent) text-(--color-accent-content)' : 'text-(--color-content-muted) hover:bg-(--color-surface-muted)'"
        :aria-pressed="model === option.value"
        @click="model = option.value"
      >{{ option.label }}</button>
      <button
        ref="customTrigger"
        type="button"
        :disabled="disabled"
        class="rounded px-2.5 py-1 text-sm font-medium transition-colors disabled:opacity-50"
        :class="model === 'custom' ? 'bg-(--color-accent) text-(--color-accent-content)' : 'text-(--color-content-muted) hover:bg-(--color-surface-muted)'"
        :aria-pressed="model === 'custom'"
        @click="openCustom"
      >Custom</button>
    </div>

    <Transition name="menu-down">
        <section
          v-if="customOpen"
          ref="customPanel"
          popover="manual"
          :style="floatingStyle"
          role="dialog"
          aria-labelledby="custom-range-title"
          class="m-0 w-[min(30rem,calc(100vw-1.5rem))] rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-4 text-(--color-content) shadow-2xl"
        >
          <h2 id="custom-range-title" class="font-semibold">Custom date range</h2>
          <p class="mt-1 text-sm text-(--color-content-muted)">Choose any period up to 366 days. Times use your local timezone.</p>
          <div class="mt-4 grid gap-4 sm:grid-cols-2">
            <UiDateTimePicker v-model="from" label="From" required />
            <UiDateTimePicker v-model="to" label="To" required />
          </div>
          <p v-if="error" class="mt-3 text-sm text-(--color-danger)" role="alert">{{ error }}</p>
          <div class="mt-5 flex justify-end gap-2">
            <UiButton variant="secondary" @click="customOpen = false">Cancel</UiButton>
            <UiButton @click="applyCustom">Apply range</UiButton>
          </div>
        </section>
    </Transition>
  </div>
</template>
