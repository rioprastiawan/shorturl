<script setup lang="ts">
definePageMeta({ middleware: 'auth' })
await navigateTo('/dashboard/system/qr-branding', { redirectCode: 301 })
</script>
