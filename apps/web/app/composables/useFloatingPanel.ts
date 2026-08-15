import type { CSSProperties, Ref } from 'vue'

interface FloatingPanelOptions {
  align?: 'start' | 'end'
  width?: 'anchor' | number
  gap?: number
  estimatedHeight?: number
}

/** Positions a teleported panel against its trigger in viewport coordinates. */
export function useFloatingPanel(
  anchor: Ref<HTMLElement | null>,
  open: Ref<boolean>,
  options: FloatingPanelOptions = {},
) {
  const style = ref<CSSProperties>({ position: 'fixed', visibility: 'hidden' })

  function update() {
    if (!import.meta.client || !open.value || !anchor.value) return
    const rect = anchor.value.getBoundingClientRect()
    const gap = options.gap ?? 6
    const viewportGap = 12
    const width = options.width === 'anchor' || options.width === undefined
      ? rect.width
      : Math.min(options.width, window.innerWidth - viewportGap * 2)
    const estimatedHeight = options.estimatedHeight ?? 280
    const below = window.innerHeight - rect.bottom - viewportGap
    const placeAbove = below < estimatedHeight && rect.top > below
    let left = options.align === 'end' ? rect.right - width : rect.left
    left = Math.max(viewportGap, Math.min(left, window.innerWidth - width - viewportGap))

    style.value = {
      position: 'fixed',
      zIndex: 100,
      width: `${width}px`,
      left: `${left}px`,
      top: placeAbove ? undefined : `${rect.bottom + gap}px`,
      bottom: placeAbove ? `${window.innerHeight - rect.top + gap}px` : undefined,
      maxHeight: `${Math.max(120, placeAbove ? rect.top - gap - viewportGap : below)}px`,
      visibility: 'visible',
    }
  }

  watch(open, async (value) => {
    if (!value) return
    style.value = { position: 'fixed', visibility: 'hidden' }
    await nextTick()
    update()
  })

  onMounted(() => {
    window.addEventListener('resize', update)
    window.addEventListener('scroll', update, true)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('resize', update)
    window.removeEventListener('scroll', update, true)
  })

  return { floatingStyle: style, updateFloating: update }
}
