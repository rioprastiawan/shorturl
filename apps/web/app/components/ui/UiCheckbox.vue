<script setup lang="ts">
const props = withDefaults(defineProps<{
  value?: string
  disabled?: boolean
}>(), {
  value: undefined,
  disabled: false,
})

const model = defineModel<boolean | string[]>({ required: true })
const checked = computed(() => Array.isArray(model.value)
  ? props.value !== undefined && model.value.includes(props.value)
  : model.value)

function update(event: Event) {
  const isChecked = (event.target as HTMLInputElement).checked
  if (!Array.isArray(model.value)) {
    model.value = isChecked
    return
  }
  if (props.value === undefined) return
  const next = new Set(model.value)
  if (isChecked) next.add(props.value)
  else next.delete(props.value)
  model.value = [...next]
}
</script>

<template>
  <label class="group flex cursor-pointer items-start gap-2.5 text-sm" :class="disabled ? 'cursor-not-allowed opacity-55' : ''">
    <input type="checkbox" class="peer sr-only" :checked="checked" :value="value" :disabled="disabled" @change="update">
    <span
      aria-hidden="true"
      class="mt-0.5 grid size-4.5 shrink-0 place-items-center rounded-[5px] border border-(--color-border-strong) bg-(--color-surface-raised) text-transparent shadow-sm transition-all group-hover:border-(--color-content-subtle) peer-focus-visible:ring-2 peer-focus-visible:ring-(--color-accent)/35 peer-focus-visible:ring-offset-2 peer-focus-visible:ring-offset-(--color-surface-raised) peer-checked:border-(--color-accent) peer-checked:bg-(--color-accent) peer-checked:text-(--color-accent-content)"
    >
      <Icon name="lucide:check" size="13" stroke-width="3" />
    </span>
    <span class="min-w-0 flex-1"><slot /></span>
  </label>
</template>
