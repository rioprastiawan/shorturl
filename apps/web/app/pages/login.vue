<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ layout: 'auth', middleware: 'guest' })

useHead({ title: 'Sign in · ShortURL' })

const { auth } = useServices()
const { branding } = useBranding()
const session = useSession()
const route = useRoute()
const setupStatus = useState<import('~/types/api').SetupStatus | null>('setup.status', () => null)

onMounted(async () => {
  try {
    setupStatus.value = await useServices().setup.status()
  } catch {
    // Keep sign-in usable if the status request itself fails.
  }
})

const email = ref('')
const password = ref('')
const code = ref('')
const needsTwoFactor = ref(false)
const pending = ref(false)

// Two kinds of failure, shown in two places: 401 is about the pair of
// credentials and belongs above the form; 422 is about one input and belongs
// under it.
const formError = ref<string | null>(null)
const errors = reactive<{ email?: string, password?: string }>({})

async function submit() {
  pending.value = true
  formError.value = null
  errors.email = undefined
  errors.password = undefined

  try {
    const user = await auth.login({ email: email.value, password: password.value, ...(needsTwoFactor.value ? { code: code.value } : {}) })
    session.set(user)

    // Honour where the auth middleware bounced us from, so a deep link
    // survives the detour through sign-in.
    const redirect = route.query.redirect
    await navigateTo(typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/dashboard')
  } catch (error) {
    if (error instanceof ApiError) {
      if (error.code === 'two_factor_required') {
        needsTwoFactor.value = true
        formError.value = null
        await nextTick()
        return
      }
      if (needsTwoFactor.value) {
        const codeError = error.field('code')
        if (codeError) { formError.value = codeError; return }
      }
      errors.email = error.field('email')
      errors.password = error.field('password')
      if (!errors.email && !errors.password) formError.value = error.message
    } else {
      formError.value = 'Could not reach the server. Check your connection and try again.'
    }
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-col gap-1">
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">
        Welcome back
      </h1>
      <p class="text-sm text-(--color-content-muted)">
        Sign in to continue to your workspace.
      </p>
    </header>

    <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
      <DashboardFormAlert v-if="formError">
        {{ formError }}
      </DashboardFormAlert>

      <UiInput
        v-model="email"
        label="Email"
        type="email"
        autocomplete="email"
        placeholder="you@example.com"
        required
        :error="errors.email"
      />

      <div v-if="needsTwoFactor" class="rounded-xl border border-(--color-border) bg-(--color-surface-muted)/60 p-3">
        <UiInput
          v-model="code"
          label="Authentication code"
          autocomplete="one-time-code"
          placeholder="6-digit code or recovery code"
          required
          hint="Open your authenticator app, or enter one of your recovery codes."
        />
      </div>

      <UiInput
        v-model="password"
        label="Password"
        type="password"
        autocomplete="current-password"
        required
        :error="errors.password"
      />

      <UiButton type="submit" :loading="pending">
        {{ needsTwoFactor ? 'Verify and sign in' : `Sign in to ${branding.app_name}` }}
      </UiButton>
    </form>

    <p v-if="setupStatus?.registration_enabled" class="text-sm text-(--color-content-muted)">
      Don't have an account?
      <NuxtLink to="/register" class="font-medium text-(--color-accent) hover:underline">
        Create one
      </NuxtLink>
    </p>
  </div>
</template>
