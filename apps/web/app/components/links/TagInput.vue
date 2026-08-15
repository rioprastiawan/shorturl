<script setup lang="ts">
import { normalizeTags } from './form'

const props = withDefaults(defineProps<{
  label?: string
  hint?: string
  error?: string
  disabled?: boolean
  maxTags?: number
  maxLength?: number
  suggestions?: string[]
}>(), {
  label: 'Tags',
  hint: 'Type a tag, then press Enter or comma.',
  error: undefined,
  disabled: false,
  maxTags: 8,
  maxLength: 24,
  suggestions: () => [],
})

const model = defineModel<string>({ required: true })
const draft = ref('')
const input = ref<HTMLInputElement | null>(null)
const tags = computed(() => normalizeTags(model.value))
const limitReached = computed(() => tags.value.length >= props.maxTags)
const focused = ref(false)
const filteredSuggestions = computed(() => {
  const query = draft.value.trim().toLowerCase()
  return props.suggestions
    .filter(tag => !tags.value.includes(tag) && (!query || tag.includes(query)))
    .slice(0, 8)
})

function update(next: string[]) {
  model.value = next.join(', ')
}

function commit(raw = draft.value) {
  const candidates = normalizeTags(raw)
  if (!candidates.length) {
    draft.value = ''
    return
  }

  const next = [...tags.value]
  for (const candidate of candidates) {
    if (next.length >= props.maxTags) break
    const tag = candidate.slice(0, props.maxLength)
    if (tag && !next.includes(tag)) next.push(tag)
  }
  update(next)
  draft.value = ''
}

function remove(tag: string) {
  if (props.disabled) return
  update(tags.value.filter(item => item !== tag))
  nextTick(() => input.value?.focus())
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Enter' || event.key === ',') {
    event.preventDefault()
    commit()
    return
  }
  if (event.key === 'Backspace' && !draft.value && tags.value.length) {
    event.preventDefault()
    remove(tags.value.at(-1)!)
  }
}

function onInput(event: Event) {
  const value = (event.target as HTMLInputElement).value
  if (value.includes(',')) {
    commit(value)
  }
}

function focusInput() {
  if (!props.disabled && !limitReached.value) input.value?.focus()
}

function selectSuggestion(tag: string) {
  commit(tag)
  nextTick(() => input.value?.focus())
}
</script>

<template>
  <div>
    <label class="mb-1.5 block text-sm font-medium text-(--color-content)">
      {{ label }}
    </label>
    <div class="relative">
    <div
      class="flex min-h-10 flex-wrap items-center gap-1.5 rounded-md border bg-(--color-surface-raised) px-2.5 py-1.5 transition-colors"
      :class="[
        error ? 'border-(--color-danger)' : 'border-(--color-border) focus-within:border-(--color-accent)',
        disabled ? 'cursor-not-allowed opacity-60' : 'cursor-text',
      ]"
      @click="focusInput"
    >
      <span
        v-for="tag in tags"
        :key="tag"
        class="inline-flex max-w-full items-center gap-1 rounded-full bg-(--color-accent)/10 px-2 py-1 text-xs font-medium text-(--color-accent)"
      >
        <span class="truncate">{{ tag }}</span>
        <button
          type="button"
          class="grid size-4 shrink-0 place-items-center rounded-full hover:bg-(--color-accent)/15"
          :disabled="disabled"
          :aria-label="`Remove ${tag}`"
          @click.stop="remove(tag)"
        >
          <Icon name="lucide:x" size="11" />
        </button>
      </span>
      <input
        v-if="!limitReached"
        ref="input"
        v-model="draft"
        type="text"
        :disabled="disabled"
        :maxlength="maxLength"
        :placeholder="tags.length ? 'Add another…' : 'campaign, social, client-a'"
        class="min-w-32 flex-1 bg-transparent px-0.5 py-1 text-sm outline-none placeholder:text-(--color-content-subtle)"
        @keydown="onKeydown"
        @input="onInput"
        @focus="focused = true"
        @blur="focused = false; commit()"
      >
    </div>
    <div
      v-if="focused && !limitReached && filteredSuggestions.length"
      class="absolute z-30 mt-1 max-h-48 w-full overflow-y-auto rounded-md border border-(--color-border) bg-(--color-surface-raised) p-1 shadow-lg"
    >
      <button
        v-for="suggestion in filteredSuggestions"
        :key="suggestion"
        type="button"
        class="flex w-full items-center gap-2 rounded px-2.5 py-2 text-left text-sm hover:bg-(--color-surface-muted)"
        @mousedown.prevent="selectSuggestion(suggestion)"
      >
        <Icon name="lucide:tag" size="13" class="text-(--color-content-subtle)" />
        {{ suggestion }}
      </button>
    </div>
    </div>
    <p v-if="error" class="mt-1.5 text-xs text-(--color-danger)" role="alert">{{ error }}</p>
    <p v-else class="mt-1.5 text-xs text-(--color-content-muted)">
      {{ limitReached ? `Maximum ${maxTags} tags reached.` : hint }}
    </p>
  </div>
</template>
