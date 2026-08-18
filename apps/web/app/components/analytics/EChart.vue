<script setup lang="ts">
import type { ECharts, EChartsCoreOption } from 'echarts/core'
import { init, use } from 'echarts/core'
import { BarChart, LineChart, MapChart, PieChart } from 'echarts/charts'
import {
  DatasetComponent,
  GridComponent,
  LegendComponent,
  PolarComponent,
  TooltipComponent,
  VisualMapComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([BarChart, LineChart, MapChart, PieChart, DatasetComponent, GridComponent, LegendComponent, PolarComponent, TooltipComponent, VisualMapComponent, CanvasRenderer])

const props = withDefaults(defineProps<{
  option: EChartsCoreOption
  height?: string
  label?: string
}>(), { height: '20rem', label: 'Analytics chart' })

const root = ref<HTMLDivElement | null>(null)
const colorMode = useColorMode()
let chart: ECharts | null = null
let observer: ResizeObserver | null = null
let resizeFrame: number | null = null

function resize() {
  if (resizeFrame !== null) cancelAnimationFrame(resizeFrame)
  resizeFrame = requestAnimationFrame(() => {
    resizeFrame = null
    chart?.resize()
  })
}

function render() {
  if (!root.value) return
  if (!chart) chart = init(root.value, undefined, { renderer: 'canvas' })
  chart.setOption(props.option, { notMerge: true })
}

onMounted(() => {
  render()
  observer = new ResizeObserver(resize)
  observer.observe(root.value!)
  window.addEventListener('resize', resize, { passive: true })
  window.addEventListener('orientationchange', resize, { passive: true })
  resize()
})

watch(() => props.option, render, { deep: true })
watch(() => colorMode.value, () => nextTick(render))

onBeforeUnmount(() => {
  observer?.disconnect()
  window.removeEventListener('resize', resize)
  window.removeEventListener('orientationchange', resize)
  if (resizeFrame !== null) cancelAnimationFrame(resizeFrame)
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div ref="root" class="min-w-0 w-full max-w-full overflow-hidden" :style="{ height }" role="img" :aria-label="label" />
</template>
