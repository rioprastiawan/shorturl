<script setup lang="ts" generic="T">
export interface DataTableColumn {
  key: string
  label: string
  align?: 'left' | 'center' | 'right'
  /** Visually hide the label while preserving it for screen readers. */
  srOnly?: boolean
}

const props = withDefaults(defineProps<{
  columns: DataTableColumn[]
  rows: T[]
  rowKey: keyof T | ((row: T) => string | number)
  minWidth?: string
  hover?: boolean
}>(), {
  minWidth: '56rem',
  hover: true,
})

function keyFor(row: T): string | number {
  if (typeof props.rowKey === 'function') return props.rowKey(row)
  return String(row[props.rowKey])
}
</script>

<template>
  <div>
    <div class="overflow-x-auto">
      <table class="w-full text-left text-sm" :style="{ minWidth }">
        <thead>
          <tr class="border-b border-(--color-border) bg-(--color-surface-muted)/45 text-xs uppercase tracking-wide text-(--color-content-subtle)">
            <th
              v-for="column in columns"
              :key="column.key"
              scope="col"
              class="px-5 py-3 font-medium"
              :class="column.align === 'right' ? 'text-right' : column.align === 'center' ? 'text-center' : 'text-left'"
            >
              <span :class="column.srOnly ? 'sr-only' : ''">{{ column.label }}</span>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="(row, index) in rows"
            :key="keyFor(row)"
            class="border-b border-(--color-border) transition-colors last:border-0"
            :class="hover ? 'hover:bg-(--color-surface-muted)' : ''"
          >
            <slot name="row" :row="row" :index="index" />
          </tr>
        </tbody>
      </table>
    </div>
    <slot name="footer" />
  </div>
</template>
