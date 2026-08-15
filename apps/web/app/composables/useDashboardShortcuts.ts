export const WORKSPACE_SHORTCUT_EVENT = 'shorturl:open-workspace-switcher'

function isTypingTarget(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  return target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName)
}

export function useDashboardShortcuts() {
  const route = useRoute()
  const createLinkModal = useCreateLinkModal()
  const helpOpen = useState('dashboard.shortcut-help', () => false)
  const commandOpen = useState('dashboard.command-palette', () => false)
  let awaitingGo = false
  let goTimer: ReturnType<typeof setTimeout> | undefined

  const destinations: Record<string, string> = {
    o: '/dashboard',
    l: '/dashboard/links',
    a: '/dashboard/analytics',
    d: '/dashboard/domains',
    m: '/dashboard/workspace/members',
    i: '/dashboard/api-keys',
    s: '/dashboard/workspace/settings',
    p: '/dashboard/account-settings',
  }

  async function focusLinkSearch() {
    if (route.path !== '/dashboard/links') await navigateTo('/dashboard/links')
    await nextTick()
    requestAnimationFrame(() => {
      document.querySelector<HTMLInputElement>('[data-shortcut-search="links"]')?.focus()
    })
  }

  function resetGo() {
    awaitingGo = false
    if (goTimer) clearTimeout(goTimer)
    goTimer = undefined
  }

  function onKeydown(event: KeyboardEvent) {
    const key = event.key.toLowerCase()

    // Browser-reserved search must be intercepted during capture, before page
    // controls or the browser handle it.
    if (event.metaKey || event.ctrlKey) {
      if (event.altKey || event.shiftKey) return
      if (key === 'k') {
        event.preventDefault()
        event.stopPropagation()
        commandOpen.value = true
      }
      return
    }

    if (event.defaultPrevented || isTypingTarget(event.target)) return
    if (event.altKey || event.shiftKey && event.key !== '?') return
    if (document.querySelector('dialog[open]')) return

    if (awaitingGo) {
      const destination = destinations[key]
      resetGo()
      if (destination) {
        event.preventDefault()
        void navigateTo(destination)
      }
      return
    }

    if (key === 'g') {
      awaitingGo = true
      goTimer = setTimeout(resetGo, 900)
    } else if (key === 'c') {
      event.preventDefault()
      createLinkModal.show()
    } else if (key === 'w') {
      event.preventDefault()
      window.dispatchEvent(new CustomEvent(WORKSPACE_SHORTCUT_EVENT))
    } else if (event.key === '?') {
      event.preventDefault()
      helpOpen.value = true
    } else if (event.key === '/') {
      event.preventDefault()
      void focusLinkSearch()
    }
  }

  onMounted(() => window.addEventListener('keydown', onKeydown, { capture: true }))
  onBeforeUnmount(() => {
    resetGo()
    window.removeEventListener('keydown', onKeydown, { capture: true })
  })

  return { helpOpen, commandOpen }
}
