<script setup lang="ts">
import type { ECharts, EChartsCoreOption } from 'echarts/core'
import { init, use } from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import {
  DatasetComponent,
  GridComponent,
  LegendComponent,
  PolarComponent,
  TooltipComponent,
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

use([BarChart, LineChart, PieChart, DatasetComponent, GridComponent, LegendComponent, PolarComponent, TooltipComponent, CanvasRenderer])

const props = withDefaults(defineProps<{
  option: EChartsCoreOption
  height?: string
  label?: string
}>(), { height: '20rem', label: 'Analytics chart' })

const root = ref<HTMLDivElement | null>(null)
const colorMode = useColorMode()
let chart: ECharts | null = null
let observer: ResizeObserver | null = null

function render() {
  if (!root.value) return
  if (!chart) chart = init(root.value, undefined, { renderer: 'canvas' })
  chart.setOption(props.option, { notMerge: true })
}

onMounted(() => {
  render()
  observer = new ResizeObserver(() => chart?.resize())
  observer.observe(root.value!)
})

watch(() => props.option, render, { deep: true })
watch(() => colorMode.value, () => nextTick(render))

onBeforeUnmount(() => {
  observer?.disconnect()
  chart?.dispose()
  chart = null
})
</script>

<template>
  <div ref="root" class="w-full" :style="{ height }" role="img" :aria-label="label" />
</template>
