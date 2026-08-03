import { ref } from 'vue'
import type { OverlayMode } from '../../common/types/api.types'
import { onEvent } from '../../common/utils/events'
import { GetOverlayMode, HideOverlay, SetOverlayMode, ShowOverlay } from '../../common/utils/wails'

export function useOverlay() {
  const mode = ref<OverlayMode>('click-through')

  async function loadMode(): Promise<void> {
    try {
      mode.value = (await GetOverlayMode()) as OverlayMode
    } catch {
      // keep default
    }
  }

  async function setMode(next: OverlayMode): Promise<void> {
    mode.value = next
    await SetOverlayMode(next)
  }

  async function show(): Promise<void> {
    await ShowOverlay()
  }

  async function hide(): Promise<void> {
    await HideOverlay()
  }

  async function toggleMode(): Promise<void> {
    await setMode(mode.value === 'click-through' ? 'interactive' : 'click-through')
  }

  onEvent('overlay:mode', payload => {
    const p = payload as { mode?: OverlayMode }
    console.log('overlay:mode =>', payload)
    if (p?.mode) {
      mode.value = p.mode
    }
  })

  return { mode, loadMode, setMode, show, hide, toggleMode }
}
