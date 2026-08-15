<script setup lang="ts">
import type { EChartsCoreOption } from 'echarts/core'
import type { TopLink, ValueStat } from '~/types/api'

const props = defineProps<{
  topLinks: TopLink[]
  devices: ValueStat[]
  hours: ValueStat[]
  weekdays: ValueStat[]
}>()

const colorMode = useColorMode()
const numberFmt = new Intl.NumberFormat(undefined)
const palette = ['#16a34a', '#2563eb', '#f59e0b', '#8b5cf6', '#ec4899', '#06b6d4', '#64748b']

function color(name: string, fallback: string) {
  colorMode.value
  if (!import.meta.client) return fallback
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function baseTooltip() {
  return {
    backgroundColor: color('--color-surface-raised', '#fff'),
    borderColor: color('--color-border', '#e2e8f0'),
    textStyle: { color: color('--color-content', '#172033') },
  }
}

const topLinksOption = computed<EChartsCoreOption>(() => {
  const rows = props.topLinks.slice(0, 7).reverse()
  return {
    grid: { top: 8, right: 28, bottom: 30, left: 118 },
    tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, ...baseTooltip() },
    xAxis: {
      type: 'value', minInterval: 1,
      axisLabel: { color: color('--color-content-muted', '#64748b') },
      splitLine: { lineStyle: { color: color('--color-border', '#e2e8f0'), type: 'dashed' } },
    },
    yAxis: {
      type: 'category', data: rows.map(link => link.title || link.slug),
      axisLabel: { color: color('--color-content-muted', '#64748b'), width: 100, overflow: 'truncate' },
      axisTick: { show: false }, axisLine: { show: false },
    },
    series: [{
      name: 'Clicks', type: 'bar', data: rows.map(link => link.clicks), barMaxWidth: 22,
      itemStyle: { color: color('--color-accent', '#16a34a'), borderRadius: [0, 5, 5, 0] },
      label: { show: true, position: 'right', color: color('--color-content-muted', '#64748b'), formatter: '{c}' },
    }],
  }
})

const devicesOption = computed<EChartsCoreOption>(() => ({
  color: palette,
  tooltip: { trigger: 'item', valueFormatter: (value: unknown) => `${numberFmt.format(Number(value))} clicks`, ...baseTooltip() },
  legend: { bottom: 0, textStyle: { color: color('--color-content-muted', '#64748b') } },
  series: [{
    name: 'Device', type: 'pie', radius: ['48%', '72%'], center: ['50%', '43%'],
    avoidLabelOverlap: true,
    itemStyle: { borderColor: color('--color-surface-raised', '#fff'), borderWidth: 3, borderRadius: 5 },
    label: { show: false }, emphasis: { label: { show: true, fontSize: 13, fontWeight: 'bold' } },
    data: props.devices.slice(0, 7).map(item => ({ name: item.value || 'Unknown', value: item.clicks })),
  }],
}))

const hoursOption = computed<EChartsCoreOption>(() => ({
  tooltip: { trigger: 'axis', ...baseTooltip() },
  polar: { radius: '70%' },
  angleAxis: {
    type: 'category', data: props.hours.map(item => item.value), startAngle: 90,
    axisLabel: { color: color('--color-content-muted', '#64748b'), interval: Math.max(0, Math.ceil(props.hours.length / 8) - 1) },
    axisLine: { lineStyle: { color: color('--color-border', '#e2e8f0') } },
  },
  radiusAxis: {
    minInterval: 1, axisLabel: { show: false },
    splitLine: { lineStyle: { color: color('--color-border', '#e2e8f0') } },
  },
  series: [{
    name: 'Clicks', type: 'bar', coordinateSystem: 'polar', data: props.hours.map(item => item.clicks),
    roundCap: true, itemStyle: { color: color('--color-accent', '#16a34a'), opacity: 0.82 },
  }],
}))

const weekdaysOption = computed<EChartsCoreOption>(() => ({
  grid: { top: 12, right: 12, bottom: 36, left: 48 },
  tooltip: { trigger: 'axis', axisPointer: { type: 'shadow' }, ...baseTooltip() },
  xAxis: {
    type: 'category', data: props.weekdays.map(item => item.value),
    axisLabel: { color: color('--color-content-muted', '#64748b'), interval: 0 },
    axisTick: { show: false }, axisLine: { lineStyle: { color: color('--color-border', '#e2e8f0') } },
  },
  yAxis: {
    type: 'value', minInterval: 1, axisLabel: { color: color('--color-content-muted', '#64748b') },
    splitLine: { lineStyle: { color: color('--color-border', '#e2e8f0'), type: 'dashed' } },
  },
  series: [{
    name: 'Clicks', type: 'bar', data: props.weekdays.map(item => item.clicks), barMaxWidth: 38,
    itemStyle: { color: color('--color-accent', '#16a34a'), borderRadius: [5, 5, 0, 0] },
  }],
}))
</script>

<template>
  <section class="grid gap-4 lg:grid-cols-2" aria-label="Analytics visual insights">
    <UiCard v-if="topLinks.length" title="Top links by clicks">
      <AnalyticsEChart :option="topLinksOption" height="19rem" label="Horizontal bar chart of top links by clicks" />
    </UiCard>
    <UiCard v-if="devices.length" title="Device share">
      <AnalyticsEChart :option="devicesOption" height="19rem" label="Donut chart of clicks by device" />
    </UiCard>
    <UiCard v-if="hours.length" title="Traffic by hour">
      <AnalyticsEChart :option="hoursOption" height="20rem" label="Polar bar chart of traffic by hour" />
    </UiCard>
    <UiCard v-if="weekdays.length" title="Traffic by weekday">
      <AnalyticsEChart :option="weekdaysOption" height="20rem" label="Bar chart of traffic by weekday" />
    </UiCard>
  </section>
</template>
