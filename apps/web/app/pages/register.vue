<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

definePageMeta({ layout: 'auth', middleware: 'guest' })

useHead({ title: 'Create account · ShortURL' })

const { auth } = useServices()
const session = useSession()
const route = useRoute()
const setupStatus = useState<import('~/types/api').SetupStatus | null>('setup.status', () => null)

const name = ref('')
const email = ref('')
const password = ref('')
const pending = ref(false)

const formError = ref<string | null>(null)
const errors = reactive<{ name?: string, email?: string, password?: string }>({})
const invitationToken = computed(() => typeof route.query.invite === 'string' ? route.query.invite : '')

onMounted(async () => {
  setupStatus.value = await useServices().setup.status()
  if (!setupStatus.value.registration_enabled && !invitationToken.value) await navigateTo('/login')
})

async function submit() {
  pending.value = true
  formError.value = null
  errors.name = undefined
  errors.email = undefined
  errors.password = undefined

  try {
    const user = await auth.register({
      name: name.value,
      email: email.value,
      password: password.value,
      ...(invitationToken.value ? { invitation_token: invitationToken.value } : {}),
    })
    // Registration signs you in server-side, so going to /login would be a
    // pointless second form.
    session.set(user)
    await navigateTo(invitationToken.value ? '/dashboard' : '/create-workspace')
  } catch (error) {
    if (error instanceof ApiError) {
      errors.name = error.field('name')
      errors.email = error.field('email')
      errors.password = error.field('password')
      const invitationError = error.field('invitation_token')
      if (!errors.name && !errors.email && !errors.password) formError.value = invitationError ?? error.message
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
        {{ invitationToken ? 'Accept invitation' : 'Create your account' }}
      </h1>
      <p class="text-sm text-(--color-content-muted)">
        {{ invitationToken ? 'Create your account to join the workspace.' : 'You will be signed in straight away.' }}
      </p>
    </header>

    <form class="flex flex-col gap-4" novalidate @submit.prevent="submit">
      <DashboardFormAlert v-if="formError">
        {{ formError }}
      </DashboardFormAlert>

      <UiInput
        v-model="name"
        label="Name"
        autocomplete="name"
        placeholder="Ada Lovelace"
        required
        :error="errors.name"
      />

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
        autocomplete="new-password"
        required
        hint="At least 10 characters"
        :error="errors.password"
      />

      <UiButton type="submit" :loading="pending">
        Create account
      </UiButton>
    </form>

    <p class="text-sm text-(--color-content-muted)">
      Already have an account?
      <NuxtLink to="/login" class="font-medium text-(--color-accent) hover:underline">
        Sign in
      </NuxtLink>
    </p>
  </div>
</template>
