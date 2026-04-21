import { ref, type Ref } from 'vue'

// useClipboard writes text to the clipboard and flashes a reactive key for
// visual feedback (e.g. "Copied!" badge). On clipboard API failure falls back
// to window.prompt so the user can copy manually.
export function useClipboard<K = boolean>(flashMs = 1500): {
  copied: Ref<K | null>
  copy: (text: string, key?: K) => Promise<boolean>
} {
  const copied = ref<K | null>(null) as Ref<K | null>

  async function copy(text: string, key?: K): Promise<boolean> {
    const flashKey = key ?? (true as unknown as K)
    try {
      await navigator.clipboard.writeText(text)
      copied.value = flashKey
      setTimeout(() => {
        if (copied.value === flashKey) copied.value = null
      }, flashMs)
      return true
    } catch {
      window.prompt('Copy manually:', text)
      return false
    }
  }

  return { copied, copy }
}
