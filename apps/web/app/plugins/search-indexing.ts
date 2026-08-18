export default defineNuxtPlugin(async () => {
  const config = useRuntimeConfig()
  let enabled = false
  try {
    const response = await $fetch<{ data: { enabled: boolean } }>(`${config.public.apiBaseUrl}/system/indexing`)
    enabled = response.data.enabled
  } catch {
    // Fail closed: an unavailable preference must not accidentally expose an
    // internal installation to indexing.
  }

  useHead({
    meta: [{
      name: 'robots',
      content: enabled ? 'index, follow' : 'noindex, nofollow, noarchive',
    }],
  })
})
