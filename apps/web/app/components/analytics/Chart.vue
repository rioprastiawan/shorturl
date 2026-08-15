<script setup lang="ts">
import type { EChartsCoreOption } from 'echarts/core'
import type { SeriesPoint } from '~/types/api'
import type { Granularity } from './types'

const props = withDefaults(defineProps<{
  points: SeriesPoint[]
  granularity: Granularity
  label?: string
}>(), { label: 'Clicks over time' })

const colorMode = useColorMode()
const numberFmt = new Intl.NumberFormat(undefined)
const hourFmt = new Intl.DateTimeFormat(undefined, { hour: 'numeric' })
const dayFmt = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })

function cssColor(name: string, fallback: string) {
  if (!import.meta.client) return fallback
  return getComputedStyle(document.documentElement).getPropertyValue(name).trim() || fallback
}

function dateLabel(value: string) {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return props.granularity === 'hour' ? hourFmt.format(date) : dayFmt.format(date)
}

const option = computed<EChartsCoreOption>(() => {
  colorMode.value
  const accent = cssColor('--color-accent', '#16a34a')
  const content = cssColor('--color-content', '#172033')
  const muted = cssColor('--color-content-muted', '#64748b')
  const border = cssColor('--color-border', '#e2e8f0')
  const raised = cssColor('--color-surface-raised', '#ffffff')

  return {
    animationDuration: 500,
    grid: { top: 22, right: 18, bottom: 48, left: 52 },
    tooltip: {
      trigger: 'axis', backgroundColor: raised, borderColor: border,
      textStyle: { color: content },
      valueFormatter: (value: unknown) => `${numberFmt.format(Number(value))} clicks`,
    },
    xAxis: {
      type: 'category', boundaryGap: false,
      data: props.points.map(point => dateLabel(point.period)),
      axisLabel: { color: muted, hideOverlap: true },
      axisLine: { lineStyle: { color: border } }, axisTick: { show: false },
    },
    yAxis: {
      type: 'value', minInterval: 1, axisLabel: { color: muted },
      splitLine: { lineStyle: { color: border, type: 'dashed' } },
    },
    series: [{
      name: 'Clicks', type: 'line', data: props.points.map(point => point.clicks),
      smooth: 0.28, symbol: props.points.length <= 31 ? 'circle' : 'none', symbolSize: 7,
      lineStyle: { width: 3, color: accent }, itemStyle: { color: accent },
      emphasis: { focus: 'series' },
      areaStyle: {
        color: {
          type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
          colorStops: [{ offset: 0, color: `${accent}55` }, { offset: 1, color: `${accent}05` }],
        },
      },
    }],
  }
})
</script>

<template>
  <AnalyticsEChart :option="option" height="21rem" :label="label" />
</template>
