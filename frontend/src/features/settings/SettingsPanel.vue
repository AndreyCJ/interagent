<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useSettings } from './useSettings'
import { useModels } from './useModels'

const { settings, error, load, save } = useSettings()
const { status, progress, downloading, error: modelError, download, refresh } = useModels()
const draft = ref({ sttModel: 'large-v3', sttLanguage: 'auto' })

onMounted(async () => {
  await load()
  if (settings.value) {
    draft.value = { sttModel: settings.value.sttModel, sttLanguage: settings.value.sttLanguage }
  }
  await refresh(draft.value.sttModel)
  await refresh('silero-vad')
})

function apply() {
  if (!settings.value) return
  save({ ...settings.value, sttModel: draft.value.sttModel, sttLanguage: draft.value.sttLanguage })
  void refresh(draft.value.sttModel)
}
</script>

<template>
  <div class="settings-panel">
    <label>
      STT model
      <select v-model="draft.sttModel">
        <option value="tiny">tiny</option>
        <option value="base">base</option>
        <option value="small">small</option>
        <option value="large-v3">large-v3</option>
        <option value="large-v3-turbo">large-v3-turbo</option>
      </select>
    </label>
    <label>
      STT language
      <select v-model="draft.sttLanguage">
        <option value="auto">auto</option>
        <option value="ru">ru</option>
        <option value="en">en</option>
      </select>
    </label>
    <button :disabled="!settings" @click="apply">Save</button>
    <span v-if="error" class="err">{{ error }}</span>

    <div class="model-status">
      <div>
        STT model ({{ draft.sttModel }}):
        <span :class="status[draft.sttModel] ? 'ok' : 'missing'">
          {{ status[draft.sttModel] ? 'installed' : 'missing' }}
        </span>
      </div>
      <div>
        VAD (silero-vad):
        <span :class="status['silero-vad'] ? 'ok' : 'missing'">
          {{ status['silero-vad'] ? 'installed' : 'missing' }}
        </span>
      </div>
      <button :disabled="downloading" @click="download">
        {{ downloading ? 'Downloading...' : 'Download models' }}
      </button>
      <div v-if="downloading" class="progress">
        <div
          class="progress-bar"
          :style="{
            width: `${Math.min(progress[draft.sttModel] ?? 0, 100)}%`,
          }"
        />
      </div>
      <span v-if="modelError" class="err">{{ modelError }}</span>
    </div>
  </div>
</template>
