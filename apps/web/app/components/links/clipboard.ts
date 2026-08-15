/**
 * Programmatic copy for places that cannot use `<UiCopyButton>` — a dropdown
 * menu item, for instance. Mirrors the component's fallback because the
 * clipboard API needs a secure context and a self-hosted install is often
 * reached over plain http on a LAN.
 */
export async function copyText(value: string): Promise<boolean> {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(value)
      return true
    }
    const el = document.createElement('textarea')
    el.value = value
    el.setAttribute('readonly', '')
    el.style.position = 'fixed'
    el.style.opacity = '0'
    document.body.appendChild(el)
    el.select()
    document.execCommand('copy')
    document.body.removeChild(el)
    return true
  } catch {
    return false
  }
}
