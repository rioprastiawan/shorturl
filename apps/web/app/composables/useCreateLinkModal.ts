export function useCreateLinkModal() {
  const open = useState('links.create-modal', () => false)
  return { open, show: () => (open.value = true), hide: () => (open.value = false) }
}
