<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ layout: 'auth', middleware: 'guest' })

useHead({ title: 'Sign in · ShortURL' })

const { auth } = useServices()
const session = useSession()
const route = useRoute()

const email = ref('')
const password = ref('')
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
    const user = await auth.login({ email: email.value, password: password.value })
    session.set(user)

    // Honour where the auth middleware bounced us from, so a deep link
    // survives the detour through sign-in.
    const redirect = route.query.redirect
    await navigateTo(typeof redirect === 'string' && redirect.startsWith('/') ? redirect : '/dashboard')
  } catch (error) {
    if (error instanceof ApiError) {
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
      <h1 class="text-xl font-semibold tracking-tight">
        Sign in
      </h1>
      <p class="text-sm text-(--color-content-muted)">
        Sign in to your ShortURL dashboard.
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

      <UiInput
        v-model="password"
        label="Password"
        type="password"
        autocomplete="current-password"
        required
        :error="errors.password"
      />

      <UiButton type="submit" :loading="pending">
        Sign in
      </UiButton>
    </form>

    <p class="text-sm text-(--color-content-muted)">
      Don't have an account?
      <NuxtLink to="/register" class="font-medium text-(--color-accent) hover:underline">
        Create one
      </NuxtLink>
    </p>
  </div>
</template>
