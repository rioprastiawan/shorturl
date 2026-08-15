import type { Ref } from 'vue'

/**
 * Calls handler when a click lands outside the element.
 *
 * Listens in the capture phase so a stopPropagation() inside some other
 * component cannot leave a dropdown stuck open.
 */
export function onClickOutside(
  target: Ref<HTMLElement | null> | Array<Ref<HTMLElement | null>>,
  handler: (event: MouseEvent) => void,
) {
  if (!import.meta.client) return

  function listener(event: MouseEvent) {
    const targets = Array.isArray(target) ? target : [target]
    const path = event.composedPath()
    if (targets.some((item) => {
      const el = item.value
      return el && (el === event.target || path.includes(el))
    })) return
    handler(event)
  }

  onMounted(() => document.addEventListener('click', listener, true))
  onBeforeUnmount(() => document.removeEventListener('click', listener, true))
}
