<script setup lang="ts">
import type { NuxtError } from '#app'

const props = defineProps<{ error: NuxtError }>()
const session = useSession()
const { branding } = useBranding()
const isDev = import.meta.dev

const statusCode = computed(() => Number(props.error.statusCode || 500))
const isNotFound = computed(() => statusCode.value === 404)
const isForbidden = computed(() => statusCode.value === 403)

const eyebrow = computed(() => isNotFound.value ? 'Lost link' : isForbidden.value ? 'Access denied' : 'Something broke')
const title = computed(() => {
  if (isNotFound.value) return 'This page came up short.'
  if (isForbidden.value) return 'You can’t open this page.'
  return 'We hit an unexpected redirect.'
})
const description = computed(() => {
  if (isNotFound.value) return 'The address may be incorrect, or the page may have moved somewhere else.'
  if (isForbidden.value) return 'Your account does not have permission to view this resource.'
  return 'Your data is safe. Try loading the page again, or head back to your workspace.'
})
const primaryTarget = computed(() => session.isAuthenticated.value ? '/dashboard' : '/')
const primaryLabel = computed(() => session.isAuthenticated.value ? 'Back to dashboard' : 'Go to home')

useHead({ title: () => `${statusCode.value} · ${branding.value.app_name}` })

async function goHome() {
  await clearError({ redirect: primaryTarget.value })
}

function retry() {
  window.location.reload()
}
</script>

<template>
  <div class="relative grid min-h-dvh place-items-center overflow-hidden bg-(--color-surface-muted) px-5 py-10 text-(--color-content)">
    <div class="pointer-events-none absolute -left-32 -top-32 size-96 rounded-full bg-(--color-accent)/10 blur-3xl" />
    <div class="pointer-events-none absolute -bottom-40 -right-28 size-[28rem] rounded-full bg-lime-400/10 blur-3xl" />

    <main class="relative w-full max-w-2xl">
      <NuxtLink to="/" class="mb-8 inline-flex items-center gap-2.5 font-bold tracking-tight">
        <BrandLogo />
      </NuxtLink>

      <section class="overflow-hidden rounded-2xl border border-(--color-border) bg-(--color-surface-raised) shadow-[0_24px_80px_rgba(15,23,42,0.10)]">
        <div class="grid gap-8 p-6 sm:p-9 md:grid-cols-[auto_1fr] md:items-center">
          <div class="relative grid size-32 place-items-center sm:size-40">
            <div class="absolute inset-0 rotate-6 rounded-[2rem] bg-(--color-accent)/10" />
            <div class="absolute inset-3 -rotate-3 rounded-3xl border border-(--color-accent)/20 bg-(--color-surface-muted)" />
            <span class="relative text-5xl font-extrabold tabular-nums tracking-tighter text-(--color-accent) sm:text-6xl">
              {{ statusCode }}
            </span>
          </div>

          <div>
            <p class="text-xs font-bold uppercase tracking-[0.18em] text-(--color-accent)">{{ eyebrow }}</p>
            <h1 class="mt-2 text-2xl font-bold tracking-tight sm:text-3xl">{{ title }}</h1>
            <p class="mt-3 max-w-lg text-sm leading-6 text-(--color-content-muted)">{{ description }}</p>

            <div class="mt-6 flex flex-wrap gap-2.5">
              <UiButton @click="goHome">
                <Icon name="lucide:layout-dashboard" size="16" /> {{ primaryLabel }}
              </UiButton>
              <UiButton v-if="!isNotFound" variant="secondary" @click="retry">Try again</UiButton>
              <UiButton v-else variant="secondary" @click="$router.back()">Go back</UiButton>
            </div>
          </div>
        </div>

        <div v-if="isDev && error.message" class="border-t border-(--color-border) bg-(--color-surface-muted)/65 px-6 py-4 sm:px-9">
          <p class="text-xs font-semibold text-(--color-content-muted)">Development detail</p>
          <code class="mt-1 block break-words text-xs text-(--color-content-subtle)">{{ error.message }}</code>
        </div>
      </section>

      <p class="mt-5 text-center text-xs text-(--color-content-subtle)">
        Error {{ statusCode }} · {{ branding.app_name }}
      </p>
    </main>
    <UiToaster />
  </div>
</template>
