<script setup lang="ts">
type SelectValue = string | number

const props = withDefaults(defineProps<{
  options: Array<{ label: string, value: SelectValue, disabled?: boolean }>
  label?: string
  hint?: string
  error?: string
  placeholder?: string
  required?: boolean
  disabled?: boolean
  inputId?: string
  size?: 'sm' | 'md'
  searchable?: boolean
  searchPlaceholder?: string
}>(), {
  label: undefined,
  hint: undefined,
  error: undefined,
  placeholder: 'Select an option',
  required: false,
  disabled: false,
  inputId: undefined,
  size: 'md',
  searchable: false,
  searchPlaceholder: 'Search options…',
})

const model = defineModel<SelectValue | null>()
const generatedId = useId()
const id = computed(() => props.inputId ?? generatedId)
const open = ref(false)
const activeIndex = ref(-1)
const root = useTemplateRef<HTMLElement>('root')
const trigger = useTemplateRef<HTMLElement>('trigger')
const list = useTemplateRef<HTMLElement>('list')
const searchInput = useTemplateRef<HTMLInputElement>('searchInput')
const searchQuery = ref('')
let typeahead = ''
let typeaheadTimer: ReturnType<typeof setTimeout> | undefined

const selectedIndex = computed(() => props.options.findIndex(option => option.value === model.value))
const selected = computed(() => props.options[selectedIndex.value])
const visibleOptions = computed(() => {
  const query = searchQuery.value.trim().toLocaleLowerCase()
  return props.options
    .map((option, index) => ({ option, index }))
    .filter(({ option }) => !query || option.label.toLocaleLowerCase().includes(query))
})

const { floatingStyle } = useFloatingPanel(trigger, open, {
  width: 'anchor',
  estimatedHeight: 272,
  // Prefer opening below and constrain the scrollable list when space is
  // limited. Flipping based on the full estimate made short selects open
  // above even though their actual content easily fit below.
  flipThreshold: 120,
})

onClickOutside([root, list], () => close())
onBeforeUnmount(() => clearTimeout(typeaheadTimer))

watch(open, async (isOpen) => {
  if (!isOpen) return
  await nextTick()
  const element = list.value
  if (element && 'showPopover' in element && !element.matches(':popover-open')) {
    element.showPopover()
  }
})

function firstEnabled(from: number, direction: 1 | -1): number {
  if (!props.options.length) return -1
  let index = from
  for (let count = 0; count < props.options.length; count++) {
    index = (index + direction + props.options.length) % props.options.length
    if (!props.options[index]?.disabled) return index
  }
  return -1
}

function show() {
  if (props.disabled) return
  searchQuery.value = ''
  open.value = true
  activeIndex.value = selectedIndex.value >= 0
    ? selectedIndex.value
    : firstEnabled(-1, 1)
  nextTick(() => {
    if (props.searchable) searchInput.value?.focus()
    else list.value?.querySelector<HTMLElement>('[data-active="true"]')?.scrollIntoView({ block: 'nearest' })
  })
}

function close() {
  open.value = false
  activeIndex.value = -1
  searchQuery.value = ''
}

function moveVisible(direction: 1 | -1) {
  const enabled = visibleOptions.value.filter(({ option }) => !option.disabled)
  if (!enabled.length) {
    activeIndex.value = -1
    return
  }
  const position = enabled.findIndex(({ index }) => index === activeIndex.value)
  const next = position < 0
    ? (direction === 1 ? 0 : enabled.length - 1)
    : (position + direction + enabled.length) % enabled.length
  activeIndex.value = enabled[next]!.index
  nextTick(() => list.value?.querySelector<HTMLElement>('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }))
}

function onSearchKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    close()
  } else if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    moveVisible(event.key === 'ArrowDown' ? 1 : -1)
  } else if (event.key === 'Enter' && activeIndex.value >= 0) {
    event.preventDefault()
    choose(activeIndex.value)
  }
}

watch(searchQuery, () => {
  if (!props.searchable) return
  activeIndex.value = visibleOptions.value.find(({ option }) => !option.disabled)?.index ?? -1
})

function choose(index: number) {
  const option = props.options[index]
  if (!option || option.disabled) return
  model.value = option.value
  close()
}

function onKeydown(event: KeyboardEvent) {
  if (props.disabled) return
  if (event.key === 'Escape') {
    close()
    return
  }
  if (event.key === 'Enter' || event.key === ' ') {
    event.preventDefault()
    if (!open.value) show()
    else if (activeIndex.value >= 0) choose(activeIndex.value)
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!open.value) show()
    else activeIndex.value = firstEnabled(activeIndex.value, event.key === 'ArrowDown' ? 1 : -1)
    nextTick(() => list.value?.querySelector<HTMLElement>('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }))
    return
  }
  if (open.value && (event.key === 'Home' || event.key === 'End')) {
    event.preventDefault()
    activeIndex.value = firstEnabled(event.key === 'Home' ? -1 : 0, event.key === 'Home' ? 1 : -1)
    return
  }
  if (event.key.length === 1 && !event.metaKey && !event.ctrlKey && !event.altKey) {
    if (props.searchable) {
      event.preventDefault()
      show()
      searchQuery.value = event.key
      return
    }
    typeahead += event.key.toLocaleLowerCase()
    clearTimeout(typeaheadTimer)
    typeaheadTimer = setTimeout(() => (typeahead = ''), 600)
    const match = props.options.findIndex(option => !option.disabled && option.label.toLocaleLowerCase().startsWith(typeahead))
    if (match >= 0) {
      if (!open.value) show()
      activeIndex.value = match
      nextTick(() => list.value?.querySelector<HTMLElement>('[data-active="true"]')?.scrollIntoView({ block: 'nearest' }))
    }
  }
}
</script>

<template>
  <div ref="root" class="relative flex min-w-0 max-w-full flex-col gap-1">
    <label v-if="label" :id="`${id}-label`" :for="id" class="text-sm font-medium">
      {{ label }}
      <span v-if="required" class="text-(--color-danger)" aria-hidden="true">*</span>
    </label>

    <button
      ref="trigger"
      :id="id"
      type="button"
      role="combobox"
      :aria-labelledby="label ? `${id}-label` : undefined"
      :aria-expanded="open"
      :aria-controls="`${id}-listbox`"
      :aria-activedescendant="open && activeIndex >= 0 ? `${id}-option-${activeIndex}` : undefined"
      :aria-invalid="error ? 'true' : undefined"
      :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
      :disabled="disabled"
      class="flex min-w-0 w-full max-w-full items-center justify-between gap-2.5 overflow-hidden rounded-md border border-(--color-border-strong) bg-(--color-surface-raised) text-left text-sm shadow-sm transition-[border-color,box-shadow,transform] duration-200 hover:border-(--color-content-subtle) focus:-translate-y-px focus:shadow-md disabled:cursor-not-allowed disabled:opacity-50"
      :class="[size === 'sm' ? 'px-2.5 py-1.5' : 'px-3 py-1.5', error ? 'border-(--color-danger)' : '']"
      @click="open ? close() : show()"
      @keydown="onKeydown"
    >
      <span class="min-w-0 flex-1 truncate" :class="selected ? '' : 'text-(--color-content-subtle)'" :title="selected?.label">
        {{ selected?.label ?? placeholder }}
      </span>
      <Icon name="lucide:chevron-down" size="16" class="shrink-0 text-(--color-content-subtle) transition-transform duration-200" :class="open ? 'rotate-180' : ''" />
    </button>

    <Transition name="menu-down">
        <div
          v-if="open"
          ref="list"
          popover="manual"
          :style="floatingStyle"
          class="m-0 flex flex-col overflow-hidden rounded-lg border border-(--color-border) bg-(--color-surface-raised) text-(--color-content) shadow-xl"
        >
        <div v-if="searchable" class="border-b border-(--color-border) p-2">
          <div class="flex items-center gap-2 rounded-lg border border-(--color-border-strong) bg-(--color-surface-muted) px-2.5">
            <Icon name="lucide:search" size="15" class="shrink-0 text-(--color-content-subtle)" />
            <input
              ref="searchInput"
              v-model="searchQuery"
              type="search"
              :placeholder="searchPlaceholder"
              class="min-w-0 flex-1 bg-transparent py-2 text-sm outline-none placeholder:text-(--color-content-subtle)"
              autocomplete="off"
              @keydown="onSearchKeydown"
            >
          </div>
        </div>
        <div :id="`${id}-listbox`" role="listbox" :aria-labelledby="label ? `${id}-label` : undefined" class="min-h-0 max-h-56 overflow-y-auto p-1.5">
        <button
          v-for="{ option, index } in visibleOptions"
          :id="`${id}-option-${index}`"
          :key="`${option.value}`"
          type="button"
          role="option"
          :aria-selected="option.value === model"
          :disabled="option.disabled"
          :data-active="index === activeIndex"
          class="flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-sm transition-colors disabled:opacity-40"
          :class="index === activeIndex ? 'bg-(--color-surface-muted)' : 'hover:bg-(--color-surface-muted)'"
          @mouseenter="activeIndex = index"
          @click="choose(index)"
        >
          <span class="min-w-0 flex-1 truncate" :title="option.label">{{ option.label }}</span>
          <Icon v-if="option.value === model" name="lucide:check" size="16" class="shrink-0 text-(--color-accent)" />
        </button>
        <p v-if="!visibleOptions.length" class="px-3 py-6 text-center text-sm text-(--color-content-muted)">
          No options found
        </p>
        </div>
        </div>
    </Transition>

    <p v-if="error" :id="`${id}-error`" class="text-xs text-(--color-danger)" role="alert">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-hint`" class="text-xs text-(--color-content-muted)">{{ hint }}</p>
  </div>
</template>
