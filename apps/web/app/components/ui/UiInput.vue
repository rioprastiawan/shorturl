<script setup lang="ts">
withDefaults(defineProps<{
  label?: string
  hint?: string
  error?: string
  type?: string
  placeholder?: string
  required?: boolean
  disabled?: boolean
  autocomplete?: string
  prefix?: string
  inputmode?: 'none' | 'text' | 'decimal' | 'numeric' | 'tel' | 'search' | 'email' | 'url'
}>(), {
  label: undefined,
  hint: undefined,
  error: undefined,
  type: 'text',
  placeholder: undefined,
  required: false,
  disabled: false,
  autocomplete: undefined,
  prefix: undefined,
  inputmode: undefined,
})

const model = defineModel<string | number | null>()
const id = useId()
</script>

<template>
  <div class="flex flex-col gap-1">
    <label v-if="label" :for="id" class="text-sm font-medium">
      {{ label }}
      <span v-if="required" class="text-(--color-danger)" aria-hidden="true">*</span>
    </label>

    <div class="flex items-stretch">
      <span
        v-if="prefix"
        class="inline-flex items-center rounded-l-md border border-r-0 border-(--color-border-strong) bg-(--color-surface-muted) px-3 text-sm text-(--color-content-muted)"
      >{{ prefix }}</span>

      <input
        :id="id"
        v-model="model"
        :type="type"
        :placeholder="placeholder"
        :required="required"
        :disabled="disabled"
        :autocomplete="autocomplete"
        :inputmode="inputmode"
        :aria-invalid="error ? 'true' : undefined"
        :aria-describedby="error ? `${id}-error` : hint ? `${id}-hint` : undefined"
        class="w-full border border-(--color-border-strong) bg-(--color-surface-raised) px-3 py-1.5 text-sm shadow-sm transition-[border-color,box-shadow,transform] duration-200 placeholder:text-(--color-content-subtle) hover:border-(--color-content-subtle) focus:-translate-y-px focus:shadow-md disabled:opacity-50"
        :class="[
          prefix ? 'rounded-r-md' : 'rounded-md',
          error ? 'border-(--color-danger)' : '',
        ]"
      >
    </div>

    <p v-if="error" :id="`${id}-error`" class="text-xs text-(--color-danger)" role="alert">
      {{ error }}
    </p>
    <p v-else-if="hint" :id="`${id}-hint`" class="text-xs text-(--color-content-muted)">
      {{ hint }}
    </p>
  </div>
</template>
