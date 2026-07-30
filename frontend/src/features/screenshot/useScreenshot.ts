import { ref } from 'vue'

export function useScreenshot() {
  const capturing = ref(false)
  const error = ref<string | null>(null)

  async function captureFull() {
    capturing.value = true
    error.value = null
  }

  async function captureRegion() {
    capturing.value = true
    error.value = null
  }

  return { capturing, error, captureFull, captureRegion }
}
