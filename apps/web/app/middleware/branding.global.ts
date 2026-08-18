/**
 * Loads branding before any page renders, on every route including `/` and
 * `/setup` where no other middleware runs.
 *
 * Without this, branding was only fetched from useBranding's onMounted hook
 * (app.vue), which by Vue's bottom-up mount order always fires after the
 * destination page has already rendered once with DEFAULT_BRANDING — a
 * visible flash before the real branding swaps in. useBranding().load()
 * already dedupes via its `loaded` state, so this is a no-op after the first
 * navigation.
 */
export default defineNuxtRouteMiddleware(async () => {
  await useBranding().load()
})
