export interface Toast {
  id: number
  message: string
  tone: 'success' | 'error' | 'info'
}

let nextId = 0

/** Transient feedback for actions that would otherwise complete silently. */
export function useToast() {
  const toasts = useState<Toast[]>('ui.toasts', () => [])

  function push(message: string, tone: Toast['tone'] = 'info', ttl = 4000) {
    const id = ++nextId
    toasts.value = [...toasts.value, { id, message, tone }]
    if (import.meta.client) {
      setTimeout(() => dismiss(id), ttl)
    }
  }

  function dismiss(id: number) {
    toasts.value = toasts.value.filter(t => t.id !== id)
  }

  return {
    toasts: readonly(toasts),
    success: (message: string) => push(message, 'success'),
    // Errors linger: they usually need reading, and often acting on.
    error: (message: string) => push(message, 'error', 7000),
    info: (message: string) => push(message, 'info'),
    dismiss,
  }
}
