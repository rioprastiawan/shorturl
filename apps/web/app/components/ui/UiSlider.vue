<script setup lang="ts">
const props = withDefaults(defineProps<{ label?: string, hint?: string, min?: number, max?: number, step?: number, suffix?: string, disabled?: boolean }>(), { label: undefined, hint: undefined, min: 0, max: 100, step: 1, suffix: '', disabled: false })
const model = defineModel<number>({ required: true })
const id = useId()
const progress = computed(() => `${Math.max(0, Math.min(100, ((model.value - props.min) / (props.max - props.min)) * 100))}%`)
</script>

<template>
  <div class="flex flex-col gap-2" :class="disabled ? 'opacity-55' : ''">
    <div class="flex items-center justify-between gap-3"><label v-if="label" :for="id" class="text-sm font-medium">{{ label }}</label><output :for="id" class="min-w-12 rounded-md bg-(--color-surface-muted) px-2 py-1 text-center text-xs font-bold tabular-nums text-(--color-content-muted)">{{ model }}{{ suffix }}</output></div>
    <input :id="id" v-model.number="model" type="range" :min="min" :max="max" :step="step" :disabled="disabled" class="ui-slider h-1.5 w-full cursor-pointer appearance-none rounded-full disabled:cursor-not-allowed" :style="{ '--slider-progress': progress }">
    <div class="flex justify-between text-[10px] tabular-nums text-(--color-content-subtle)"><span>{{ min }}{{ suffix }}</span><span>{{ max }}{{ suffix }}</span></div>
    <p v-if="hint" class="text-xs text-(--color-content-muted)">{{ hint }}</p>
  </div>
</template>

<style scoped>
.ui-slider { background: linear-gradient(to right, var(--color-accent) var(--slider-progress), var(--color-border) var(--slider-progress)); }
.ui-slider::-webkit-slider-thumb { width: 1.1rem; height: 1.1rem; appearance: none; border: 3px solid var(--color-surface-raised); border-radius: 999px; background: var(--color-accent); box-shadow: 0 1px 5px rgb(15 23 42 / .25); }
.ui-slider::-moz-range-thumb { width: .8rem; height: .8rem; border: 3px solid var(--color-surface-raised); border-radius: 999px; background: var(--color-accent); box-shadow: 0 1px 5px rgb(15 23 42 / .25); }
</style>
