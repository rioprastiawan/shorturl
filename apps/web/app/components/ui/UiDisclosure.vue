<script setup lang="ts">
withDefaults(defineProps<{
  title: string
  description?: string
  icon?: string
  disabled?: boolean
}>(), {
  description: undefined,
  icon: 'lucide:settings-2',
  disabled: false,
})

const open = defineModel<boolean>({ default: false })
const panelId = useId()
</script>

<template>
  <section
    class="overflow-hidden rounded-xl border bg-(--color-surface-raised) transition-[border-color,box-shadow] duration-200"
    :class="open ? 'border-(--color-accent)/35 shadow-sm' : 'border-(--color-border)'"
  >
    <button
      type="button"
      class="group flex w-full items-center gap-3 p-3.5 text-left transition-colors hover:bg-(--color-surface-muted)/55 disabled:cursor-not-allowed disabled:opacity-55 sm:p-4"
      :disabled="disabled"
      :aria-expanded="open"
      :aria-controls="panelId"
      @click="open = !open"
    >
      <span
        class="grid size-9 shrink-0 place-items-center rounded-lg transition-colors"
        :class="open ? 'bg-(--color-accent) text-(--color-accent-content)' : 'bg-(--color-surface-muted) text-(--color-content-muted) group-hover:text-(--color-content)'"
      >
        <Icon :name="icon" size="17" />
      </span>
      <span class="min-w-0 flex-1">
        <span class="block text-sm font-semibold">{{ title }}</span>
        <span v-if="description" class="mt-0.5 block text-xs leading-relaxed text-(--color-content-muted)">{{ description }}</span>
      </span>
      <span class="grid size-7 shrink-0 place-items-center rounded-full text-(--color-content-subtle) transition-[background-color,color,transform] group-hover:bg-(--color-surface-muted) group-hover:text-(--color-content)" :class="open ? 'rotate-180' : ''">
        <Icon name="lucide:chevron-down" size="16" />
      </span>
    </button>

    <div
      :id="panelId"
      class="grid transition-[grid-template-rows,opacity] duration-300 ease-out"
      :class="open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0'"
      :aria-hidden="!open"
      :inert="!open"
    >
      <div class="min-h-0 overflow-hidden">
        <div class="border-t border-(--color-border) bg-(--color-surface-muted)/25 p-3.5 sm:p-4">
          <slot />
        </div>
      </div>
    </div>
  </section>
</template>
