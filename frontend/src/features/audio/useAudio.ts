import { ref } from 'vue'
import {
  IsListening,
  OpenPermissionSettings,
  StartListening,
  StopListening,
} from '../../common/utils/wails'
import { onEvent } from '../../common/utils/events'

interface AppErrorPayload {
  stage?: string
  permission?: string
}

export function useAudio() {
  const isListening = ref(false)
  const error = ref<string | null>(null)
  const busy = ref(false)
  const errorPermission = ref<string | null>(null)

  onEvent('app:error', (payload: unknown) => {
    const p = (payload ?? {}) as AppErrorPayload
    if (p.permission) errorPermission.value = p.permission
  })

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
    await OpenPermissionSettings(errorPermission.value ?? 'screen-recording')
  }

  return { isListening, error, busy, errorPermission, load, toggle, openSettings }
}
