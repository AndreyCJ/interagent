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

  async function load() {
    try {
      isListening.value = await IsListening()
    } catch {
      // keep default
    }
  }

  async function toggle() {
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

  async function openSettings() {
    const target = (error.value ?? '').includes('microphone') ? 'microphone' : 'screen-recording'
    await OpenPermissionSettings(target)
  }

  return { isListening, error, busy, load, toggle, openSettings }
}
