<script setup lang="ts">
import type { DomainStatus, SslStatus } from '~/types/api'

/**
 * DNS and SSL states are rendered by the same component but never in the same
 * badge — plan §39 requires them shown separately, because a domain can be
 * verified while its certificate is still being issued.
 */
const props = defineProps<{
  kind: 'dns' | 'ssl'
  status: DomainStatus | SslStatus | string
}>()

type Tone = 'neutral' | 'success' | 'warning' | 'danger' | 'info'

const tones: Record<string, Tone> = {
  active: 'success',
  pending: 'warning',
  verifying: 'warning',
  failed: 'danger',
  disabled: 'neutral',
}

const dnsLabels: Record<string, string> = {
  active: 'Verified',
  pending: 'Pending',
  verifying: 'Verifying',
  failed: 'Failed',
  disabled: 'Disabled',
}

const sslLabels: Record<string, string> = {
  active: 'Active',
  pending: 'Issuing',
  failed: 'Failed',
}

const tone = computed<Tone>(() => tones[props.status] ?? 'neutral')
const label = computed(() => {
  const table = props.kind === 'ssl' ? sslLabels : dnsLabels
  return table[props.status] ?? props.status
})
const prefix = computed(() => (props.kind === 'ssl' ? 'SSL' : 'DNS'))
</script>

<template>
  <UiBadge :tone="tone" dot>
    <span class="text-(--color-content-subtle)">{{ prefix }}</span>
    {{ label }}
  </UiBadge>
</template>
