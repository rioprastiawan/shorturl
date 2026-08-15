<script setup lang="ts">
import { ApiError } from '~/composables/useApi'

definePageMeta({ layout: 'auth', middleware: 'auth' })
useHead({ title: 'Create your workspace · ShortURL' })

const ws = useWorkspaces()
const name = ref('')
const pending = ref(false)
const errorMessage = ref<string>()

async function submit() {
  if (pending.value) return
  const value = name.value.trim()
  if (!value) {
    errorMessage.value = 'Enter a workspace name.'
    return
  }
  pending.value = true
  errorMessage.value = undefined
  try {
    await ws.create(value)
    await navigateTo('/dashboard')
  } catch (error) {
    errorMessage.value = error instanceof ApiError
      ? (error.field('name') ?? error.message)
      : 'Could not create the workspace.'
  } finally {
    pending.value = false
  }
}
</script>

<template>
  <div class="flex flex-col gap-6">
    <header class="flex flex-col gap-1">
      <h1 class="text-2xl font-bold tracking-tight sm:text-3xl">Create your first workspace</h1>
      <p class="text-sm text-(--color-content-muted)">
        A workspace keeps your links, domains, members, and API keys together.
      </p>
    </header>
    <form class="flex flex-col gap-4" @submit.prevent="submit">
      <UiInput
        v-model="name"
        label="Workspace name"
        placeholder="My Workspace"
        required
        :disabled="pending"
        :error="errorMessage"
      />
      <UiButton type="submit" :loading="pending">Create workspace</UiButton>
    </form>

    <div class="flex items-center gap-3 text-xs uppercase tracking-wide text-(--color-content-subtle)">
      <span class="h-px flex-1 bg-(--color-border)" />
      or explore first
      <span class="h-px flex-1 bg-(--color-border)" />
    </div>
    <WorkspaceDemoCreator block />
  </div>
</template>
