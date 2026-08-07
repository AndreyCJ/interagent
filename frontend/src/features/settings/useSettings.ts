import { ref } from 'vue'
import type { AppSettings } from '../../common/types/api.types'
import { GetSettings, SaveSettings } from '../../common/utils/wails'

export function useSettings() {
  const settings = ref<AppSettings | null>(null)
  const error = ref<string | null>(null)

  async function load() {
    try {
      settings.value = (await GetSettings()) as unknown as AppSettings
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
