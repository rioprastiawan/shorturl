<script setup lang="ts">
import { ApiError } from '~/composables/useApi'

const open = defineModel<boolean>({ default: false })
const { auth } = useServices()
const toast = useToast()
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')
const saving = ref(false)
const errors = reactive<{ current?: string, next?: string, confirm?: string }>({})

function reset() {
  currentPassword.value = ''
  newPassword.value = ''
  confirmPassword.value = ''
  errors.current = undefined
  errors.next = undefined
  errors.confirm = undefined
}

watch(open, (isOpen) => {
  if (isOpen || !saving.value) reset()
})

async function changePassword() {
  errors.current = undefined
  errors.next = undefined
  errors.confirm = undefined
  if (newPassword.value !== confirmPassword.value) {
    errors.confirm = 'Passwords do not match'
    return
  }

  saving.value = true
  try {
    await auth.changePassword({ current_password: currentPassword.value, new_password: newPassword.value })
    open.value = false
    toast.success('Password changed. Other sessions were signed out.')
  } catch (error) {
    if (error instanceof ApiError) {
      errors.current = error.field('current_password')
      errors.next = error.field('new_password')
      if (!errors.current && !errors.next) toast.error(error.message)
    } else toast.error('Could not change your password')
  } finally {
    saving.value = false
  }
}
</script>

<template>
  <UiModal v-model="open" title="Change your password" description="Use at least 10 characters. Every other signed-in session will be ended.">
    <form class="flex flex-col gap-4" @submit.prevent="changePassword">
      <UiPasswordInput v-model="currentPassword" label="Current password" autocomplete="current-password" required :disabled="saving" :error="errors.current" />
      <UiPasswordInput v-model="newPassword" label="New password" autocomplete="new-password" required hint="At least 10 characters" :disabled="saving" :error="errors.next" />
      <UiPasswordInput v-model="confirmPassword" label="Confirm new password" autocomplete="new-password" required :disabled="saving" :error="errors.confirm" />
    </form>
    <template #actions>
      <UiButton variant="secondary" :disabled="saving" @click="open = false">Cancel</UiButton>
      <UiButton :loading="saving" @click="changePassword">Update password</UiButton>
    </template>
  </UiModal>
</template>
