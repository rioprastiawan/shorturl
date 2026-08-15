<script setup lang="ts">
import { ApiError } from '~/composables/useApi'
import { useServices } from '~/services'

/**
 * First-run wizard (plan §23).
 *
 * No auth middleware: there is by definition nobody to authenticate yet. The
 * guard is the status check below — once an install is configured this route
 * must be a dead end, or it becomes a way to ask the server to create a
 * second administrator.
 */
definePageMeta({ layout: 'auth' })

useHead({ title: 'Set up ShortURL' })

const { setup } = useServices()
const session = useSession()
const setupComplete = useState<boolean | null>('setup.completed', () => null)

const TOTAL_STEPS = 4

const checking = ref(true)
const step = ref(1)
const pending = ref(false)
const done = ref(false)
const formError = ref<string | null>(null)

const form = reactive({
  deployment_mode: 'internal' as 'internal' | 'public',
  name: '',
  email: '',
  password: '',
  workspace_name: 'My Workspace',
})

const errors = reactive<Record<string, string | undefined>>({})

onMounted(async () => {
  try {
    const status = await setup.status()
    setupComplete.value = status.completed
    if (status.completed) {
      await navigateTo('/login')
      return
    }
  } catch {
    // If the status endpoint is unreachable the wizard cannot be trusted to be
    // safe to run, so send the operator to sign-in rather than risk a second
    // administrator on a configured install.
    await navigateTo('/login')
    return
  } finally {
    checking.value = false
  }
})

function clearErrors() {
  for (const key of Object.keys(errors)) errors[key] = undefined
  formError.value = null
}

/**
 * Client-side mirror of the server's rules (auth.ValidateAccount and the
 * workspace name bounds). It exists so a typo in the email on step 2 is caught
 * on step 2, rather than as a 422 after the user has finished step 3 and can
 * no longer see the field that is wrong.
 */
function validateStep(target: number): boolean {
  clearErrors()

  if (target === 3) {
    const name = form.name.trim()
    if (!name) errors.name = 'Enter your name'
    else if (name.length < 2) errors.name = 'Must be at least 2 characters'
    else if (name.length > 120) errors.name = 'Must be at most 120 characters'

    const email = form.email.trim()
    if (!email) errors.email = 'Enter your email address'
    else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) errors.email = 'Must be a valid email address'

    if (form.password.length < 10) errors.password = 'Must be at least 10 characters'
  }

  if (target === 4) {
    const workspace = form.workspace_name.trim()
    if (!workspace) errors.workspace_name = 'Enter a workspace name'
    else if (workspace.length < 2) errors.workspace_name = 'Must be at least 2 characters'
    else if (workspace.length > 120) errors.workspace_name = 'Must be at most 120 characters'
  }

  return Object.values(errors).every(value => !value)
}

function next() {
  if (!validateStep(step.value)) return
  step.value = Math.min(step.value + 1, TOTAL_STEPS)
}

function back() {
  clearErrors()
  step.value = Math.max(step.value - 1, 1)
}

async function goToDashboard() {
  await navigateTo('/dashboard')
}

async function submit() {
  // Re-check both input steps: the user may have gone back and broken step 2
  // after it was already accepted. Step 2 first, so that if both are wrong the
  // errors left on screen are the ones on the step we send them back to.
  if (!validateStep(3)) {
    step.value = 3
    return
  }
  if (!validateStep(4)) return

  pending.value = true
  formError.value = null
  try {
    const result = await setup.complete({
      name: form.name.trim(),
      email: form.email.trim(),
      password: form.password,
      workspace_name: form.workspace_name.trim(),
      deployment_mode: form.deployment_mode,
    })

    // The server issues the session cookie as part of setup, so the new admin
    // is already signed in.
    setupComplete.value = true
    session.set(result.user)
    done.value = true
  } catch (error) {
    if (error instanceof ApiError) {
      errors.name = error.field('name')
      errors.email = error.field('email')
      errors.password = error.field('password')
      errors.workspace_name = error.field('workspace_name')

      if (errors.name || errors.email || errors.password) {
        step.value = 3
        formError.value = 'Check the administrator details.'
      } else if (errors.workspace_name) {
        formError.value = null
      } else if (error.code === 'setup_completed') {
        formError.value = 'This install has already been set up. Redirecting you to sign in…'
        await navigateTo('/login')
      } else {
        formError.value = error.message
      }
    } else {
      formError.value = 'Could not reach the server. Check your connection and try again.'
    }
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div v-if="checking" class="mx-auto flex w-full max-w-md flex-col gap-5" role="status" aria-label="Loading setup">
    <UiSkeleton height="1.75rem" width="55%" />
    <UiSkeleton height="0.9rem" width="85%" />
    <UiSkeleton height="8rem" rounded="lg" />
  </div>

  <!-- Completion note. Shown before leaving the wizard because a fresh install
       has no domain, and link creation is blocked until one is verified. -->
  <div v-else-if="done" class="flex flex-col gap-6">
    <header class="flex flex-col gap-1">
      <h1 class="text-xl font-semibold tracking-tight">
        You're all set
      </h1>
      <p class="text-sm text-(--color-content-muted)">
        Signed in as {{ form.email.trim() }}.
      </p>
    </header>

    <DashboardFormAlert tone="info">
      Next: connect a domain. A fresh install has none, and every short link
      needs a verified domain to live on.
    </DashboardFormAlert>

    <div class="flex flex-col gap-2">
      <UiButton to="/dashboard/domains">
        Connect a domain
      </UiButton>
      <UiButton variant="secondary" @click="goToDashboard">
        Go to the dashboard
      </UiButton>
    </div>
  </div>

  <div v-else class="flex flex-col gap-6">
    <header class="flex flex-col gap-1">
      <p class="text-xs font-medium uppercase tracking-wide text-(--color-content-subtle)">
        Step {{ step }} of {{ TOTAL_STEPS }}
      </p>
      <h1 class="text-xl font-semibold tracking-tight">
        {{ step === 1 ? 'Welcome to ShortURL' : step === 2 ? 'Installation mode' : step === 3 ? 'Administrator account' : 'First workspace' }}
      </h1>
    </header>

    <!-- A bar rather than a number alone: it is the only signal of how much is
         left, and the wizard cannot be abandoned halfway. -->
    <ol class="flex gap-1.5" aria-label="Setup progress">
      <li
        v-for="n in TOTAL_STEPS"
        :key="n"
        class="h-1 flex-1 rounded-full"
        :class="n <= step ? 'bg-(--color-accent)' : 'bg-(--color-border)'"
        :aria-current="n === step ? 'step' : undefined"
      />
    </ol>

    <form class="flex flex-col gap-4" novalidate @submit.prevent="step === TOTAL_STEPS ? submit() : next()">
      <DashboardFormAlert v-if="formError">
        {{ formError }}
      </DashboardFormAlert>

      <p v-if="step === 1" class="text-sm text-(--color-content-muted)">
        This wizard runs once. It creates the administrator account you will
        sign in with and the first workspace to keep your links and domains in.
        Everything else — domains, links, team members — is set up from the
        dashboard afterwards.
      </p>

      <template v-if="step === 2">
        <label class="flex cursor-pointer gap-3 rounded-lg border border-(--color-border) p-4">
          <input v-model="form.deployment_mode" type="radio" value="internal" class="mt-1">
          <span>
            <strong class="block text-sm">Internal / private</strong>
            <span class="block text-xs text-(--color-content-muted)">Public registration is disabled. People join a workspace through invitation links.</span>
          </span>
        </label>
        <label class="flex cursor-pointer gap-3 rounded-lg border border-(--color-border) p-4">
          <input v-model="form.deployment_mode" type="radio" value="public" class="mt-1">
          <span>
            <strong class="block text-sm">Public / SaaS</strong>
            <span class="block text-xs text-(--color-content-muted)">Anyone may register and create their own workspace.</span>
          </span>
        </label>
      </template>

      <template v-if="step === 3">
        <UiInput
          v-model="form.name"
          label="Name"
          autocomplete="name"
          placeholder="Ada Lovelace"
          required
          :error="errors.name"
        />
        <UiInput
          v-model="form.email"
          label="Email"
          type="email"
          autocomplete="email"
          placeholder="you@example.com"
          required
          :error="errors.email"
        />
        <UiInput
          v-model="form.password"
          label="Password"
          type="password"
          autocomplete="new-password"
          required
          hint="At least 10 characters"
          :error="errors.password"
        />
      </template>

      <template v-if="step === 4">
        <UiInput
          v-model="form.workspace_name"
          label="Workspace name"
          placeholder="My Workspace"
          required
          hint="A workspace groups your domains, links, and team. You can add more later."
          :error="errors.workspace_name"
        />
      </template>

      <div class="flex gap-2">
        <UiButton v-if="step > 1" variant="secondary" :disabled="pending" @click="back">
          Back
        </UiButton>
        <UiButton type="submit" :loading="pending" class="flex-1">
          {{ step === TOTAL_STEPS ? 'Finish setup' : 'Continue' }}
        </UiButton>
      </div>
    </form>
  </div>
</template>
