import { ref } from 'vue'
import { onEvent } from '../../common/utils/events'
import { DownloadSTTModel, GetModelStatus } from '../../common/utils/wails'

export function useModels() {
  const status = ref<Record<string, boolean>>({})
  const progress = ref<Record<string, number>>({})
  const downloading = ref(false)
  const error = ref<string | null>(null)

  async function download(): Promise<void> {
    if (downloading.value) return
    downloading.value = true
    error.value = null
    try {
      await DownloadSTTModel()
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    } finally {
      downloading.value = false
    }
  }

  async function refresh(model: string): Promise<void> {
    try {
      const s = (await GetModelStatus(model)) as {
        Installed?: boolean
        installed?: boolean
      }
      if (s.Installed ?? s.installed) status.value[model] = true
    } catch {
      // keep previous state
    }
  }

  onEvent('model:download-progress', payload => {
    const p = payload as { model?: string; received?: number }
    if (!p.model || p.received === undefined) return
    progress.value[p.model] = p.received
  })

  onEvent('model:downloaded', payload => {
    const p = payload as { model?: string }
    if (!p.model) return
    status.value[p.model] = true
    progress.value[p.model] = 100
  })

  onEvent('app:error', payload => {
    const p = payload as { stage?: string; error?: string }
    if (p.stage === 'models' && p.error) error.value = p.error
  })

  return { status, progress, downloading, error, download, refresh }
}
