<script setup lang="ts">
const props = withDefaults(defineProps<{
  label?: string
  hint?: string
  error?: string
  disabled?: boolean
  required?: boolean
}>(), {
  label: undefined,
  hint: undefined,
  error: undefined,
  disabled: false,
  required: false,
})

const model = defineModel<string>({ default: '' })
const id = useId()
const open = ref(false)
const root = useTemplateRef<HTMLElement>('root')
const trigger = useTemplateRef<HTMLElement>('trigger')
const panel = useTemplateRef<HTMLElement>('panel')
const { floatingStyle } = useFloatingPanel(trigger, open, {
  width: 336,
  estimatedHeight: 440,
})
const shownMonth = ref(startOfMonth(parseValue(model.value) ?? new Date()))

const hours = Array.from({ length: 24 }, (_, value) => ({ value: `${value}`.padStart(2, '0'), label: `${value}`.padStart(2, '0') }))
const minutes = Array.from({ length: 60 }, (_, value) => ({ value: `${value}`.padStart(2, '0'), label: `${value}`.padStart(2, '0') }))
const weekdays = ['Su', 'Mo', 'Tu', 'We', 'Th', 'Fr', 'Sa']

function parseValue(value?: string | null): Date | null {
  if (!value) return null
  const match = /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2})/.exec(value)
  if (!match) return null
  const [, year, month, day, hour, minute] = match
  const date = new Date(Number(year), Number(month) - 1, Number(day), Number(hour), Number(minute))
  return Number.isNaN(date.getTime()) ? null : date
}

function startOfMonth(date: Date) {
  return new Date(date.getFullYear(), date.getMonth(), 1)
}

function pad(value: number) {
  return `${value}`.padStart(2, '0')
}

function serialize(date: Date) {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())}T${pad(date.getHours())}:${pad(date.getMinutes())}`
}

function sameDay(left: Date | null, right: Date) {
  return !!left && left.getFullYear() === right.getFullYear() && left.getMonth() === right.getMonth() && left.getDate() === right.getDate()
}

const selected = computed(() => parseValue(model.value))
const monthLabel = computed(() => new Intl.DateTimeFormat('en', { month: 'long', year: 'numeric' }).format(shownMonth.value))
const displayValue = computed(() => selected.value
  ? new Intl.DateTimeFormat('en', { dateStyle: 'medium', timeStyle: 'short' }).format(selected.value)
  : 'Select date and time')
const selectedHour = computed<string>({
  get: () => pad(selected.value?.getHours() ?? 0),
  set: value => updateTime(Number(value), selected.value?.getMinutes() ?? 0),
})
const selectedMinute = computed<string>({
  get: () => pad(selected.value?.getMinutes() ?? 0),
  set: value => updateTime(selected.value?.getHours() ?? 0, Number(value)),
})

const days = computed(() => {
  const first = shownMonth.value
  const start = new Date(first.getFullYear(), first.getMonth(), 1 - first.getDay())
  return Array.from({ length: 42 }, (_, index) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + index))
})

onClickOutside([root, panel], () => (open.value = false))

watch(open, async (isOpen) => {
  if (!isOpen) return
  await nextTick()
  const element = panel.value
  if (element && 'showPopover' in element && !element.matches(':popover-open')) {
    element.showPopover()
  }
})

watch(model, value => {
  const date = parseValue(value)
  if (date) shownMonth.value = startOfMonth(date)
})

function selectDay(day: Date) {
  const current = selected.value
  const next = new Date(day.getFullYear(), day.getMonth(), day.getDate(), current?.getHours() ?? 0, current?.getMinutes() ?? 0)
  model.value = serialize(next)
}

function updateTime(hour: number, minute: number) {
  const base = selected.value ?? new Date()
  const next = new Date(base.getFullYear(), base.getMonth(), base.getDate(), hour, minute)
  model.value = serialize(next)
}

function moveMonth(offset: number) {
  shownMonth.value = new Date(shownMonth.value.getFullYear(), shownMonth.value.getMonth() + offset, 1)
}

function useToday() {
  const now = new Date()
  now.setSeconds(0, 0)
  model.value = serialize(now)
  shownMonth.value = startOfMonth(now)
}
</script>

<template>
  <div ref="root" class="relative flex flex-col gap-1.5">
    <label v-if="label" :id="`${id}-label`" :for="id" class="text-sm font-medium">
      {{ label }} <span v-if="required" class="text-(--color-danger)" aria-hidden="true">*</span>
    </label>
    <button
      ref="trigger"
      :id="id"
      type="button"
      :disabled="disabled"
      :aria-expanded="open"
      aria-haspopup="dialog"
      :aria-labelledby="label ? `${id}-label` : undefined"
      :aria-invalid="error ? 'true' : undefined"
      class="flex w-full items-center justify-between gap-3 rounded-lg border border-(--color-border-strong) bg-(--color-surface-raised) px-3.5 py-2 text-left text-sm shadow-sm transition-[border-color,box-shadow,transform] duration-200 hover:border-(--color-content-subtle) focus:-translate-y-px focus:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
      :class="error ? 'border-(--color-danger)' : ''"
      @click="open = !open"
      @keydown.esc="open = false"
    >
      <span :class="selected ? '' : 'text-(--color-content-subtle)'">{{ displayValue }}</span>
      <Icon name="lucide:calendar-days" size="17" class="shrink-0 text-(--color-content-subtle)" />
    </button>

    <Transition name="menu-down">
        <div v-if="open" ref="panel" popover="manual" role="dialog" aria-label="Choose date and time" :style="floatingStyle" class="m-0 overflow-y-auto rounded-xl border border-(--color-border) bg-(--color-surface-raised) p-3 text-(--color-content) shadow-2xl">
        <div class="mb-3 flex items-center justify-between">
          <button type="button" class="grid size-8 place-items-center rounded-lg hover:bg-(--color-surface-muted)" aria-label="Previous month" @click="moveMonth(-1)"><Icon name="lucide:chevron-left" size="17" /></button>
          <p class="text-sm font-semibold">{{ monthLabel }}</p>
          <button type="button" class="grid size-8 place-items-center rounded-lg hover:bg-(--color-surface-muted)" aria-label="Next month" @click="moveMonth(1)"><Icon name="lucide:chevron-right" size="17" /></button>
        </div>
        <div class="grid grid-cols-7 text-center text-[11px] font-semibold text-(--color-content-subtle)">
          <span v-for="weekday in weekdays" :key="weekday" class="py-1">{{ weekday }}</span>
        </div>
        <div class="grid grid-cols-7 gap-0.5">
          <button
            v-for="day in days"
            :key="day.toISOString()"
            type="button"
            class="grid aspect-square place-items-center rounded-lg text-xs transition-colors hover:bg-(--color-surface-muted)"
            :class="[
              day.getMonth() === shownMonth.getMonth() ? '' : 'text-(--color-content-subtle) opacity-55',
              sameDay(selected, day) ? 'bg-(--color-accent)! font-bold text-(--color-accent-content)!' : '',
              sameDay(new Date(), day) && !sameDay(selected, day) ? 'ring-1 ring-inset ring-(--color-accent)' : '',
            ]"
            @click="selectDay(day)"
          >{{ day.getDate() }}</button>
        </div>
        <div class="mt-3 flex items-end gap-2 border-t border-(--color-border) pt-3">
          <div class="min-w-0 flex-1"><UiSelect v-model="selectedHour" label="Hour" :options="hours" size="sm" /></div>
          <span class="pb-2 text-sm font-bold">:</span>
          <div class="min-w-0 flex-1"><UiSelect v-model="selectedMinute" label="Minute" :options="minutes" size="sm" /></div>
        </div>
        <div class="mt-3 flex items-center justify-between">
          <button type="button" class="text-xs font-semibold text-(--color-content-muted) hover:text-(--color-content)" @click="useToday">Today</button>
          <div class="flex gap-2">
            <button v-if="model" type="button" class="rounded-lg px-2.5 py-1.5 text-xs font-semibold text-(--color-danger) hover:bg-(--color-danger)/8" @click="model = ''">Clear</button>
            <button type="button" class="rounded-lg bg-(--color-accent) px-3 py-1.5 text-xs font-semibold text-(--color-accent-content)" @click="open = false">Done</button>
          </div>
        </div>
        </div>
    </Transition>
    <p v-if="error" class="text-xs text-(--color-danger)" role="alert">{{ error }}</p>
    <p v-else-if="hint" class="text-xs text-(--color-content-muted)">{{ hint }}</p>
  </div>
</template>
