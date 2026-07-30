import { ref } from 'vue'

export function useAudio() {
  const isListening = ref(false)
  const error = ref<string | null>(null)

  async function start() {
    isListening.value = true
    error.value = null
  }

  async function stop() {
    isListening.value = false
  }

  return { isListening, error, start, stop }
}
