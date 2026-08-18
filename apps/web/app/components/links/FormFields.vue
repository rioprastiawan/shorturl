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
  tagSuggestions?: string[]
}>(), {
  errors: () => ({}),
  hasPassword: false,
  disabled: false,
  advancedOpen: false,
  tagSuggestions: () => [],
})

const model = defineModel<LinkFormModel>({ required: true })
const advancedExpanded = ref(props.advancedOpen)

watch(() => props.advancedOpen, value => {
  if (value) advancedExpanded.value = true
})

const selectedDomain = computed(() =>
  props.domains.find(d => d.id === model.value.domain_id) ?? props.domains[0] ?? null)

const slugPrefix = computed(() =>
  selectedDomain.value ? `${selectedDomain.value.hostname}/` : undefined)

const domainOptions = computed(() => props.domains.map(domain => ({
  value: domain.id,
  label: `${domain.hostname}${domain.is_default ? ' (default)' : ''}`,
})))
</script>

<template>
  <div class="flex flex-col gap-4">
    <!-- Domain -->
    <UiSelect v-model="model.domain_id" label="Domain" required :options="domainOptions" :disabled="disabled" :error="errors.domain_id" />

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

    <LinksTagInput
      v-model="model.tags"
      label="Tags"
      :disabled="disabled"
      hint="Type a tag, then press Enter or comma. Up to 8 tags."
      :error="errors.tags"
      :suggestions="tagSuggestions"
    />

    <div>
      <label class="mb-1.5 block text-sm font-medium text-(--color-content)">Internal notes</label>
      <textarea
        v-model="model.notes"
        rows="3"
        maxlength="1000"
        :disabled="disabled"
        placeholder="Campaign context, owner, or anything teammates should know…"
        class="w-full resize-y rounded-md border border-(--color-border-strong) bg-(--color-surface-raised) px-3 py-2 text-sm shadow-sm transition-[border-color,box-shadow,transform] placeholder:text-(--color-content-subtle) hover:border-(--color-content-subtle) focus:-translate-y-px focus:shadow-md disabled:opacity-60"
      />
      <p class="mt-1 text-xs text-(--color-content-muted)">Only visible to workspace members.</p>
    </div>

    <!-- The common case is three fields; everything else is opt-in. -->
    <UiDisclosure v-model="advancedExpanded" title="Advanced options" description="Configure redirect behavior, expiration, password protection, and click limits." icon="lucide:sliders-horizontal">
      <div class="flex flex-col gap-4">
        <UiSelect
          v-model="model.redirect_type"
          label="Redirect type"
          :options="REDIRECT_TYPES"
          :disabled="disabled"
          :error="errors.redirect_type"
          hint="302 keeps the destination changeable; 301 is cached by browsers, sometimes forever."
        />

        <UiDateTimePicker
          v-model="model.expires_at"
          label="Expiration"
          :disabled="disabled"
          hint="After this moment the link stops resolving. Leave empty for no expiry."
          :error="errors.expires_at"
        />

        <UiPasswordInput
          v-model="model.password"
          label="Password"
          autocomplete="new-password"
          :disabled="disabled || model.remove_password"
          :placeholder="hasPassword ? 'Type a new password to replace the current one' : undefined"
          :hint="hasPassword
            ? 'This link is password protected. Leave empty to keep the current password.'
            : 'Visitors must enter this before being redirected.'"
          :error="errors.password"
        />

        <UiCheckbox v-if="hasPassword" v-model="model.remove_password" :disabled="disabled">
          Remove the password from this link
        </UiCheckbox>

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
    </UiDisclosure>
  </div>
</template>
