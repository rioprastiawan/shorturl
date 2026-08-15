<script setup lang="ts">
import type { DnsInstructions } from '~/types/api'

/**
 * The "Expected DNS" table from plan §39.
 *
 * A table rather than prose: the user is copying four values into a DNS panel
 * with Type / Name / Value columns of its own, and a paragraph forces them to
 * work out which word goes in which box.
 */
const props = defineProps<{ instructions: DnsInstructions }>()

interface Row {
  purpose: string
  note?: string
  type: string
  name: string
  value: string
  /** Guidance text such as "the IPv4 address of …" is not a copyable value. */
  copyable: boolean
}

const rows = computed<Row[]>(() => {
  const out: Row[] = []
  const v = props.instructions.verification
  if (v) {
    out.push({
      purpose: 'Ownership',
      note: 'Proves the domain is yours.',
      type: v.type,
      name: v.name,
      value: v.value,
      copyable: true,
    })
  }
  const routing = props.instructions.routing ?? []
  routing.forEach((record, i) => {
    out.push({
      purpose: i === 0 ? 'Routing' : 'Routing — alternative',
      note: i === 0
        ? 'Points visitors at this installation.'
        : 'Use only if your provider cannot host the record above.',
      type: record.type,
      name: record.name,
      value: record.value,
      copyable: !record.value.includes(' '),
    })
  })
  return out
})
</script>

<template>
  <div class="overflow-x-auto">
    <table class="w-full min-w-[36rem] text-left text-sm">
      <caption class="sr-only">
        DNS records required for this domain
      </caption>
      <thead>
        <tr class="border-b border-(--color-border) text-xs uppercase tracking-wide text-(--color-content-subtle)">
          <th scope="col" class="py-2 pr-3 font-medium">
            Purpose
          </th>
          <th scope="col" class="py-2 pr-3 font-medium">
            Type
          </th>
          <th scope="col" class="py-2 pr-3 font-medium">
            Name
          </th>
          <th scope="col" class="py-2 font-medium">
            Value
          </th>
        </tr>
      </thead>
      <tbody>
        <tr v-for="row in rows" :key="`${row.type}-${row.name}-${row.value}`" class="border-b border-(--color-border) align-top last:border-0">
          <td class="py-3 pr-3">
            <div class="font-medium">
              {{ row.purpose }}
            </div>
            <p v-if="row.note" class="mt-0.5 max-w-[14rem] text-xs text-(--color-content-muted)">
              {{ row.note }}
            </p>
          </td>
          <td class="py-3 pr-3">
            <code class="font-mono text-xs">{{ row.type }}</code>
          </td>
          <td class="py-3 pr-3">
            <UiCopyButton :value="row.name" show-value label="Copy" />
          </td>
          <td class="py-3">
            <UiCopyButton v-if="row.copyable" :value="row.value" show-value label="Copy" />
            <span v-else class="text-xs text-(--color-content-muted)">{{ row.value }}</span>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>
