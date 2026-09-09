<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useSettings } from './useSettings'

const { settings, error, load, save } = useSettings()
const draft = ref({ sttModel: 'base', sttLanguage: 'auto' })

onMounted(async () => {
  await load()
  if (settings.value) {
    draft.value = { sttModel: settings.value.sttModel, sttLanguage: settings.value.sttLanguage }
  }
})

function apply() {
  if (!settings.value) return
  save({ ...settings.value, sttModel: draft.value.sttModel, sttLanguage: draft.value.sttLanguage })
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
  </div>
</template>
