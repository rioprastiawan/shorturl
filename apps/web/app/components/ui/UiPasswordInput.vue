<script setup lang="ts">
withDefaults(defineProps<{
  label?: string
  hint?: string
  error?: string
  placeholder?: string
  required?: boolean
  disabled?: boolean
  autocomplete?: string
}>(), {
  label: undefined,
  hint: undefined,
  error: undefined,
  placeholder: undefined,
  required: false,
  disabled: false,
  autocomplete: undefined,
})

const model = defineModel<string>({ required: true })
const id = useId()
const visible = ref(false)
</script>

<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :for="id" class="text-sm font-medium">
      {{ label }}
      <span v-if="required" class="text-(--color-danger)" aria-hidden="true">*</span>
    </label>

    <div
      class="flex items-center overflow-hidden rounded-md border bg-(--color-surface-raised) shadow-sm transition-[border-color,box-shadow,transform] focus-within:-translate-y-px focus-within:shadow-md focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-(--color-accent)"
      :class="error ? 'border-(--color-danger)' : 'border-(--color-border-strong) hover:border-(--color-content-subtle)'"
    >
      <input
        :id="id"
        v-model="model"
        :type="visible ? 'text' : 'password'"
        :placeholder="placeholder"
        :required="required"
        :disabled="disabled"
        :autocomplete="autocomplete"
        :aria-invalid="error ? 'true' : undefined"
        :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
        class="min-w-0 flex-1 bg-transparent px-3 py-1.5 text-sm outline-none placeholder:text-(--color-content-subtle) disabled:opacity-50"
      >
      <button
        type="button"
        class="grid size-9 shrink-0 place-items-center text-(--color-content-muted) transition-colors hover:bg-(--color-surface-muted) hover:text-(--color-content) focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50"
        :disabled="disabled"
        :aria-label="visible ? 'Hide password' : 'Show password'"
        :title="visible ? 'Hide password' : 'Show password'"
        :aria-pressed="visible"
        @click="visible = !visible"
      >
        <Icon :name="visible ? 'lucide:eye-off' : 'lucide:eye'" size="17" />
      </button>
    </div>

    <p v-if="error" :id="`${id}-error`" class="text-xs text-(--color-danger)" role="alert">{{ error }}</p>
    <p v-else-if="hint" :id="`${id}-hint`" class="text-xs text-(--color-content-muted)">{{ hint }}</p>
  </div>
</template>
