// maskKey shortens a project key for display, showing only the first 8 chars
// followed by an ellipsis. Shared by the projects list and form so the masked
// copy-to-clipboard affordance stays identical in both places.
export function maskKey(key?: string): string {
  if (!key) return ''
  return key.length <= 8 ? key : key.slice(0, 8) + '…'
}
