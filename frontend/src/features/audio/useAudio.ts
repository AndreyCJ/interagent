import { ref } from 'vue'
import {
  IsListening,
  OpenPermissionSettings,
  StartListening,
  StopListening,
} from '../../common/utils/wails'

export function useAudio() {
  const isListening = ref(false)
  const error = ref<string | null>(null)
  const busy = ref(false)

  async function load(): Promise<void> {
    try {
      isListening.value = await IsListening()
    } catch {
      // keep default
    }
  }

  async function toggle(): Promise<void> {
    if (busy.value) return
    busy.value = true
    error.value = null
    try {
      if (isListening.value) {
        await StopListening()
      } else {
        await StartListening()
      }
      isListening.value = !isListening.value
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      busy.value = false
    }
  }

  async function openSettings(): Promise<void> {
    await OpenPermissionSettings('screen-recording')
  }

  return { isListening, error, busy, load, toggle, openSettings }
}
