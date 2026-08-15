<script setup lang="ts">
import type { EChartsCoreOption } from 'echarts/core'
import type { ValueStat } from '~/types/api'
import { registerMap } from 'echarts/core'
import countries from 'i18n-iso-countries'
import { feature } from 'topojson-client'
import world from 'world-atlas/countries-110m.json'

const props = defineProps<{ countries: ValueStat[] }>()
const colorMode = useColorMode()

// The atlas uses ISO numeric IDs. Keeping that identifier avoids fragile
// country-name aliases (United States vs United States of America, etc.).
const geo = feature(world as never, (world as any).objects.countries) as any
for (const country of geo.features) country.properties.iso_n3 = String(country.id).padStart(3, '0')
registerMap('analytics-world', geo)

function cssColor(name: string, fallback: string) {
  colorMode.value
  if (!import.meta.client) return fallback
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

const mapped = computed(() => props.countries.flatMap((item) => {
  const numeric = countries.alpha2ToNumeric(item.value.toUpperCase())
  return numeric ? [{ name: numeric, value: item.clicks, code: item.value.toUpperCase() }] : []
}))
const max = computed(() => Math.max(1, ...mapped.value.map(item => item.value)))

const option = computed<EChartsCoreOption>(() => ({
  tooltip: {
    trigger: 'item',
    backgroundColor: cssColor('--color-surface-raised', '#fff'),
    borderColor: cssColor('--color-border', '#e2e8f0'),
    textStyle: { color: cssColor('--color-content', '#172033') },
    formatter: (params: any) => params.data
      ? `${countries.getName(params.data.code, 'en') || params.data.code}<br><strong>${Number(params.value).toLocaleString()}</strong> clicks`
      : 'No clicks',
  },
  visualMap: {
    min: 0, max: max.value, calculable: false, orient: 'horizontal', left: 'center', bottom: 0,
    text: ['More', 'Less'], textStyle: { color: cssColor('--color-content-muted', '#64748b') },
    inRange: { color: ['#dcfce7', '#86efac', '#22c55e', '#15803d'] },
  },
  series: [{
    name: 'Clicks', type: 'map', map: 'analytics-world', nameProperty: 'iso_n3', roam: true,
    top: 4, bottom: 45, data: mapped.value,
    itemStyle: { areaColor: cssColor('--color-surface-muted', '#f1f5f9'), borderColor: cssColor('--color-border-strong', '#cbd5e1'), borderWidth: 0.6 },
    emphasis: { label: { show: false }, itemStyle: { areaColor: cssColor('--color-accent', '#16a34a') } },
    select: { disabled: true },
  }],
}))
</script>

<template>
  <UiCard title="Clicks around the world" description="Drag to pan and scroll to zoom. Country data comes from the request edge.">
    <AnalyticsEChart :option="option" height="25rem" label="World map showing clicks by country" />
  </UiCard>
</template>
