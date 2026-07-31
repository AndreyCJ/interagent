import { ref } from 'vue'
import type { AppSettings } from '../../types'

export function useSettings() {
  const settings = ref<AppSettings | null>(null)
  const error = ref<string | null>(null)

  async function load() {}

  async function save(s: AppSettings) {
    settings.value = s
  }

  return { settings, error, load, save }
}
