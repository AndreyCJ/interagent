import { ref } from 'vue'
import type { AppSettings, Shortcut } from '../../common/types/api.types'
import { GetSettings, SaveSettings } from '../../common/utils/wails'

export function useSettings() {
  const settings = ref<AppSettings | null>(null)
  const error = ref<string | null>(null)

  async function load() {
    try {
      const raw = (await GetSettings()) as unknown as Record<string, unknown>
      settings.value = {
        theme: (raw.theme ?? raw.Theme) as AppSettings['theme'],
        language: (raw.language ?? raw.Language) as string,
        shortcuts: (raw.shortcuts ?? raw.Shortcuts) as Shortcut[],
        autoStartListening: (raw.autoStartListening ?? raw.AutoStartListening) as boolean,
        sttModel: (raw.sttModel ?? raw.SttModel) as string,
        sttLanguage: (raw.sttLanguage ?? raw.SttLanguage) as string,
      }
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  }

  async function save(s: AppSettings) {
    try {
      await SaveSettings(s as unknown as Parameters<typeof SaveSettings>[0])
      settings.value = s
      error.value = null
    } catch (e) {
      error.value = e instanceof Error ? e.message : String(e)
    }
  }

  return { settings, error, load, save }
}
