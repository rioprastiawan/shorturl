<script setup lang="ts">
const { branding } = useBranding()
</script>

<template>
  <div class="grid min-h-dvh bg-(--color-surface-raised) lg:grid-cols-[minmax(22rem,0.9fr)_1.1fr]">
    <aside class="relative hidden overflow-hidden bg-(--color-shell) p-10 text-(--color-shell-content) transition-colors lg:flex lg:flex-col lg:justify-between xl:p-14">
      <div class="absolute -right-24 -top-24 size-80 rounded-full bg-(--color-shell-accent)/15 blur-2xl" />
      <div class="absolute -bottom-32 -left-20 size-96 rounded-full bg-(--color-accent)/20 blur-3xl" />
      <NuxtLink to="/" class="relative flex items-center gap-3 text-xl font-bold tracking-tight">
        <BrandLogo inverse />
      </NuxtLink>
      <div class="relative max-w-lg">
        <p class="mb-5 text-xs font-bold uppercase tracking-[0.2em] text-(--color-shell-accent)">{{ branding.tagline }}</p>
        <h1 class="text-4xl font-bold leading-tight tracking-tight xl:text-5xl">{{ branding.login_heading }}</h1>
        <p class="mt-5 max-w-md text-base leading-relaxed opacity-60">{{ branding.login_description }}</p>
      </div>
      <div class="relative text-xs opacity-50">
        <p>{{ branding.footer_text }}</p>
        <nav v-if="branding.support_url || branding.documentation_url || branding.privacy_url || branding.terms_url" class="mt-2 flex flex-wrap gap-x-3 gap-y-1" aria-label="Support and legal">
          <a v-if="branding.support_url" :href="branding.support_url" target="_blank" rel="noopener">Support</a>
          <a v-if="branding.documentation_url" :href="branding.documentation_url" target="_blank" rel="noopener">Docs</a>
          <a v-if="branding.privacy_url" :href="branding.privacy_url" target="_blank" rel="noopener">Privacy</a>
          <a v-if="branding.terms_url" :href="branding.terms_url" target="_blank" rel="noopener">Terms</a>
        </nav>
        <p v-if="branding.show_powered_by && branding.app_name !== 'ShortURL'" class="mt-1">Powered by ShortURL</p>
      </div>
    </aside>

    <main class="relative grid min-h-dvh place-items-center p-5 sm:p-8">
      <NuxtLink to="/" class="absolute left-5 top-5 flex items-center gap-2.5 font-bold tracking-tight lg:hidden">
        <BrandLogo />
      </NuxtLink>
      <div class="w-full max-w-md pt-16 lg:pt-0"><slot /></div>
    </main>
    <UiToaster />
  </div>
</template>
