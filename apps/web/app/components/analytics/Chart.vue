<script setup lang="ts">
import type { SeriesPoint } from '~/types/api'
import type { Granularity } from './types'

/**
 * Clicks over time, drawn as inline SVG.
 *
 * No chart library: the plan forbids new dependencies, and a single series
 * with a linear scale is about forty lines of arithmetic. Bars up to 32
 * buckets, a line beyond that — 90 daily bars are 3px wide and unreadable.
 */
const props = withDefaults(defineProps<{
  points: SeriesPoint[]
  granularity: Granularity
  /** Screen-reader summary; the visual chart is decorative on its own. */
  label?: string
}>(), {
  label: 'Clicks over time',
})

const W = 720
const H = 220
const PAD_L = 48
const PAD_R = 12
const PAD_T = 16
const PAD_B = 32
const plotW = W - PAD_L - PAD_R
const plotH = H - PAD_T - PAD_B

const hourFmt = new Intl.DateTimeFormat(undefined, { hour: 'numeric' })
const dayFmt = new Intl.DateTimeFormat(undefined, { month: 'short', day: 'numeric' })
const fullFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium', timeStyle: 'short' })
const fullDayFmt = new Intl.DateTimeFormat(undefined, { dateStyle: 'medium' })
const numberFmt = new Intl.NumberFormat(undefined)

/** A round upper bound, and never zero — the y-scale divides by this. */
function niceMax(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 1
  const magnitude = 10 ** Math.floor(Math.log10(value))
  for (const step of [1, 2, 2.5, 5, 10]) {
    if (value <= magnitude * step) return magnitude * step
  }
  return magnitude * 10
}

const count = computed(() => props.points.length)
const band = computed(() => (count.value > 0 ? plotW / count.value : plotW))
const useBars = computed(() => count.value <= 32)

const maxClicks = computed(() =>
  props.points.reduce((acc, p) => Math.max(acc, p.clicks), 0))
const scaleMax = computed(() => niceMax(maxClicks.value))
const total = computed(() => props.points.reduce((acc, p) => acc + p.clicks, 0))

function xCenter(index: number): number {
  return PAD_L + (index + 0.5) * band.value
}

function y(value: number): number {
  return PAD_T + plotH * (1 - value / scaleMax.value)
}

const bars = computed(() => props.points.map((point, i) => {
  const width = Math.max(1, band.value * 0.7)
  const top = y(point.clicks)
  return {
    key: point.period,
    x: xCenter(i) - width / 2,
    y: top,
    width,
    // A zero bar still gets a hairline so the bucket is visibly present.
    height: Math.max(point.clicks > 0 ? 1 : 0, PAD_T + plotH - top),
  }
}))

const linePath = computed(() =>
  props.points.map((p, i) => `${i === 0 ? 'M' : 'L'}${xCenter(i).toFixed(2)},${y(p.clicks).toFixed(2)}`).join(' '))

const areaPath = computed(() => {
  if (!count.value) return ''
  const baseline = PAD_T + plotH
  const first = xCenter(0).toFixed(2)
  const last = xCenter(count.value - 1).toFixed(2)
  return `${linePath.value} L${last},${baseline} L${first},${baseline} Z`
})

const yTicks = computed(() => {
  const max = scaleMax.value
  const values = max >= 2 ? [0, max / 2, max] : [0, max]
  return values.map(v => ({ value: v, y: y(v), text: numberFmt.format(v) }))
})

function shortLabel(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  return props.granularity === 'hour' ? hourFmt.format(d) : dayFmt.format(d)
}

function fullLabel(iso: string): string {
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return props.granularity === 'hour' ? fullFmt.format(d) : fullDayFmt.format(d)
}

/** At most six x labels, however many buckets there are. */
const xLabels = computed(() => {
  const step = Math.max(1, Math.ceil(count.value / 6))
  return props.points
    .map((point, i) => ({ i, point }))
    .filter(({ i }) => i % step === 0)
    .map(({ i, point }) => ({ key: point.period, x: xCenter(i), text: shortLabel(point.period) }))
})

const hovered = ref<number | null>(null)
const readout = computed(() => {
  const i = hovered.value
  if (i === null) return null
  const point = props.points[i]
  if (!point) return null
  return `${fullLabel(point.period)} · ${numberFmt.format(point.clicks)} ${point.clicks === 1 ? 'click' : 'clicks'}`
})

const summary = computed(() =>
  `${props.label}: ${numberFmt.format(total.value)} clicks across ${count.value} buckets, peaking at ${numberFmt.format(maxClicks.value)}.`)
</script>

<template>
  <div>
    <div class="mb-2 flex items-baseline justify-between gap-4">
      <p class="text-sm text-(--color-content-muted)">
        <span class="font-medium text-(--color-content)">{{ numberFmt.format(total) }}</span>
        clicks in this range
      </p>
      <p class="min-h-5 text-xs text-(--color-content-muted)" role="status" aria-live="polite">
        {{ readout }}
      </p>
    </div>

    <svg
      :viewBox="`0 0 ${W} ${H}`"
      class="h-auto w-full"
      role="img"
      :aria-label="summary"
      @mouseleave="hovered = null"
    >
      <!-- Grid and y-axis -->
      <g>
        <line
          v-for="tick in yTicks"
          :key="`grid-${tick.value}`"
          :x1="PAD_L"
          :x2="W - PAD_R"
          :y1="tick.y"
          :y2="tick.y"
          stroke="var(--color-border)"
          stroke-width="1"
        />
        <text
          v-for="tick in yTicks"
          :key="`ylabel-${tick.value}`"
          :x="PAD_L - 8"
          :y="tick.y + 4"
          text-anchor="end"
          font-size="11"
          fill="var(--color-content-subtle)"
        >{{ tick.text }}</text>
      </g>

      <!-- Baseline, drawn last of the chrome so it sits above the grid -->
      <line
        :x1="PAD_L"
        :x2="W - PAD_R"
        :y1="PAD_T + plotH"
        :y2="PAD_T + plotH"
        stroke="var(--color-border-strong)"
        stroke-width="1"
      />

      <!-- Series -->
      <g v-if="useBars">
        <rect
          v-for="(bar, i) in bars"
          :key="bar.key"
          :x="bar.x"
          :y="bar.y"
          :width="bar.width"
          :height="bar.height"
          rx="1"
          fill="var(--color-accent)"
          :opacity="hovered === null || hovered === i ? 1 : 0.45"
        />
      </g>
      <g v-else>
        <path :d="areaPath" fill="var(--color-accent)" opacity="0.12" />
        <path
          :d="linePath"
          fill="none"
          stroke="var(--color-accent)"
          stroke-width="2"
          stroke-linejoin="round"
          stroke-linecap="round"
        />
        <circle
          v-if="hovered !== null && points[hovered]"
          :cx="xCenter(hovered)"
          :cy="y(points[hovered]!.clicks)"
          r="3.5"
          fill="var(--color-accent)"
        />
      </g>

      <!-- x-axis labels -->
      <text
        v-for="labelPoint in xLabels"
        :key="`x-${labelPoint.key}`"
        :x="labelPoint.x"
        :y="H - 10"
        text-anchor="middle"
        font-size="11"
        fill="var(--color-content-subtle)"
      >{{ labelPoint.text }}</text>

      <!-- Transparent hit areas: full-height so a zero bucket is still hoverable -->
      <rect
        v-for="(point, i) in points"
        :key="`hit-${point.period}`"
        :x="xCenter(i) - band / 2"
        :y="PAD_T"
        :width="band"
        :height="plotH"
        fill="transparent"
        tabindex="0"
        class="focus:outline-none"
        @mouseenter="hovered = i"
        @focus="hovered = i"
        @blur="hovered = null"
      >
        <title>{{ fullLabel(point.period) }}: {{ numberFmt.format(point.clicks) }}</title>
      </rect>
    </svg>
  </div>
</template>
