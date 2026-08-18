<script setup lang="ts">
definePageMeta({ middleware: ['auth', 'workspace-admin'] })
await navigateTo('/dashboard/system/qr-branding', { redirectCode: 301 })
</script>
