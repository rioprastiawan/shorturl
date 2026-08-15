<template>
  <div class="grid min-h-dvh bg-(--color-surface-raised) lg:grid-cols-[minmax(22rem,0.9fr)_1.1fr]">
    <aside class="relative hidden overflow-hidden bg-[#172033] p-10 text-white transition-colors dark:bg-[#080c12] lg:flex lg:flex-col lg:justify-between xl:p-14">
      <div class="absolute -right-24 -top-24 size-80 rounded-full bg-[#84cc16]/15 blur-2xl" />
      <div class="absolute -bottom-32 -left-20 size-96 rounded-full bg-[#16a34a]/20 blur-3xl" />
      <NuxtLink to="/" class="relative flex items-center gap-3 text-xl font-bold tracking-tight">
        <span class="grid size-11 place-items-center rounded-xl bg-[#84cc16] text-[#172033]"><Icon name="lucide:link-2" size="22" /></span>
        ShortURL
      </NuxtLink>
      <div class="relative max-w-lg">
        <p class="mb-5 text-xs font-bold uppercase tracking-[0.2em] text-[#a3e635]">Simple links. Clear insights.</p>
        <h1 class="text-4xl font-bold leading-tight tracking-tight xl:text-5xl">Make every link easier to share and understand.</h1>
        <p class="mt-5 max-w-md text-base leading-relaxed text-white/60">Create branded short links, understand your audience, and keep your whole team working in one place.</p>
      </div>
      <p class="relative text-xs text-white/35">Secure, self-hosted, and built for your team.</p>
    </aside>

    <main class="relative grid min-h-dvh place-items-center p-5 sm:p-8">
      <NuxtLink to="/" class="absolute left-5 top-5 flex items-center gap-2.5 font-bold tracking-tight lg:hidden">
        <span class="grid size-9 place-items-center rounded-xl bg-(--color-accent) text-white"><Icon name="lucide:link-2" size="18" /></span>
        ShortURL
      </NuxtLink>
      <div class="w-full max-w-md pt-16 lg:pt-0"><slot /></div>
    </main>
    <UiToaster />
  </div>
</template>
