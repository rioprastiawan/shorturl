/** Restrict management pages to workspace owners and admins. */
export default defineNuxtRouteMiddleware(async () => {
  const workspaces = useWorkspaces()
  await workspaces.load()

  if (workspaces.role.value === 'member') {
    return navigateTo('/dashboard', { replace: true })
  }
})
