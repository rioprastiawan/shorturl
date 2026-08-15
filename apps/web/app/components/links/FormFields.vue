<script setup lang="ts">
import type { Domain } from '~/types/api'
import type { LinkFormErrors, LinkFormModel } from './form'
import { REDIRECT_TYPES } from './form'

/**
 * The link form's fields, shared by create and edit so the two screens cannot
 * drift apart. The parent owns submission: it decides what a changed field
 * means, which differs between POST (omit empties) and PATCH (send null).
 */
const props = withDefaults(defineProps<{
  domains: Domain[]
  errors?: LinkFormErrors
  /** Edit only: the link already has a password, so offer to remove it. */
  hasPassword?: boolean
  disabled?: boolean
  /** Open the advanced block on load when the link already uses those fields. */
  advancedOpen?: boolean
}>(), {
  errors: () => ({}),
  hasPassword: false,
  disabled: false,
  advancedOpen: false,
})

const model = defineModel<LinkFormModel>({ required: true })

const domainId = useId()
const redirectId = useId()

const selectedDomain = computed(() =>
  props.domains.find(d => d.id === model.value.domain_id) ?? props.domains[0] ?? null)

const slugPrefix = computed(() =>
  selectedDomain.value ? `${selectedDomain.value.hostname}/` : undefined)

const fieldClass = 'w-full rounded-md border border-(--color-border-strong) bg-transparent px-3 py-2 text-sm transition-colors disabled:opacity-50'
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- Domain -->
    <div class="flex flex-col gap-1.5">
      <label :for="domainId" class="text-sm font-medium">
        Domain
        <span class="text-(--color-danger)" aria-hidden="true">*</span>
      </label>
      <select
        :id="domainId"
        v-model="model.domain_id"
        :disabled="disabled"
        :class="[fieldClass, errors.domain_id ? 'border-(--color-danger)' : '']"
        :aria-invalid="errors.domain_id ? 'true' : undefined"
      >
        <option v-for="d in domains" :key="d.id" :value="d.id">
          {{ d.hostname }}{{ d.is_default ? ' (default)' : '' }}
        </option>
      </select>
      <p v-if="errors.domain_id" class="text-xs text-(--color-danger)" role="alert">
        {{ errors.domain_id }}
      </p>
    </div>

    <UiInput
      v-model="model.destination_url"
      label="Destination URL"
      type="url"
      required
      :disabled="disabled"
      placeholder="https://example.com/a/very/long/url"
      :error="errors.destination_url"
    />

    <UiInput
      v-model="model.slug"
      label="Custom slug"
      :prefix="slugPrefix"
      :disabled="disabled"
      placeholder="spring-sale"
      hint="Leave empty to generate a random one."
      :error="errors.slug"
    />

    <UiInput
      v-model="model.title"
      label="Title"
      :disabled="disabled"
      placeholder="Spring sale landing page"
      hint="Optional. Leave empty to use the destination page title automatically."
      :error="errors.title"
    />

    <!-- The common case is three fields; everything else is opt-in. -->
    <details
      class="rounded-md border border-(--color-border) bg-(--color-surface-muted) px-4 py-3"
      :open="advancedOpen"
    >
      <summary class="cursor-pointer text-sm font-medium">
        Advanced options
      </summary>

      <div class="mt-4 flex flex-col gap-4">
        <div class="flex flex-col gap-1.5">
          <label :for="redirectId" class="text-sm font-medium">Redirect type</label>
          <select
            :id="redirectId"
            v-model="model.redirect_type"
            :disabled="disabled"
            :class="[fieldClass, errors.redirect_type ? 'border-(--color-danger)' : '']"
          >
            <option v-for="t in REDIRECT_TYPES" :key="t.value" :value="t.value">
              {{ t.label }}
            </option>
          </select>
          <p v-if="errors.redirect_type" class="text-xs text-(--color-danger)" role="alert">
            {{ errors.redirect_type }}
          </p>
          <p v-else class="text-xs text-(--color-content-muted)">
            302 keeps the destination changeable; 301 is cached by browsers, sometimes forever.
          </p>
        </div>

        <UiInput
          v-model="model.expires_at"
          label="Expiration"
          type="datetime-local"
          :disabled="disabled"
          hint="After this moment the link stops resolving. Leave empty for no expiry."
          :error="errors.expires_at"
        />

        <UiInput
          v-model="model.password"
          label="Password"
          type="password"
          autocomplete="new-password"
          :disabled="disabled || model.remove_password"
          :placeholder="hasPassword ? 'Type a new password to replace the current one' : undefined"
          :hint="hasPassword
            ? 'This link is password protected. Leave empty to keep the current password.'
            : 'Visitors must enter this before being redirected.'"
          :error="errors.password"
        />

        <label v-if="hasPassword" class="flex items-center gap-2 text-sm">
          <input
            v-model="model.remove_password"
            type="checkbox"
            :disabled="disabled"
            class="size-4 rounded border-(--color-border-strong)"
          >
          Remove the password from this link
        </label>

        <UiInput
          v-model="model.max_clicks"
          label="Click limit"
          type="number"
          :disabled="disabled"
          placeholder="e.g. 500"
          hint="The link stops resolving once it reaches this many clicks."
          :error="errors.max_clicks"
        />
      </div>
    </details>
  </div>
</template>
