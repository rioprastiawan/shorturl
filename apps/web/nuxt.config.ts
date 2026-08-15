import tailwindcss from '@tailwindcss/vite'

// In production Traefik puts the dashboard and the Go API on one origin. The
// dev proxy reproduces that so the browser always talks to a single origin and
// the API never needs CORS.
const devProxyTarget = process.env.NUXT_DEV_PROXY_TARGET || 'http://localhost:8080'

export default defineNuxtConfig({
  compatibilityDate: '2026-08-15',

  // The dashboard is a pure API client: every page needs an authenticated
  // session and none of it benefits from server rendering or SEO. Running as
  // an SPA keeps the production image to static assets and removes a Node
  // process from the self-hosted stack.
  ssr: false,

  devtools: { enabled: true },

  css: ['~/assets/css/main.css'],

  // Nuxt scans app/composables and app/utils by default. The API service
  // layer lives in app/services (plan §37), so it has to be added explicitly
  // or every page needs a manual import of useServices.
  imports: {
    dirs: ['services'],
  },

  vite: {
    plugins: [tailwindcss()],
  },

  nitro: {
    devProxy: {
      '/api': { target: `${devProxyTarget}/api`, changeOrigin: true },
      '/health': { target: `${devProxyTarget}/health`, changeOrigin: true },
    },
  },

  runtimeConfig: {
    public: {
      apiBaseUrl: process.env.NUXT_PUBLIC_API_BASE_URL || '/api/v1',
    },
  },

  app: {
    head: {
      title: 'ShortURL',
      htmlAttrs: { lang: 'en' },
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      ],
    },
  },
})
