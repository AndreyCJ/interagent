<script setup lang="ts">
import { onMounted, ref } from 'vue'
import MicButton from '../audio/MicButton.vue'
import { useAudio } from '../audio/useAudio'
import { useOverlay } from './useOverlay'
import SettingsPanel from '../settings/SettingsPanel.vue'

const { mode, loadMode: loadOverlayMode, toggleMode: toggleOverlayMode, show, hide } = useOverlay()
const { isListening, busy, error, load: loadAudio, toggle: toggleAudio, openSettings } = useAudio()
const showSettings = ref(false)

onMounted(() => {
  loadOverlayMode()
  loadAudio()
})
</script>

<template>
  <div class="controls">
    <MicButton :listening="isListening" :busy="busy" @toggle="toggleAudio" />
    <span v-if="error" class="err">{{ error }}</span>
    <button v-if="error" @click="openSettings">Open Settings</button>
    <button @click="toggleOverlayMode">Mode: {{ mode }}</button>
    <button @click="show">Show</button>
    <button @click="hide">Hide</button>
    <button @click="showSettings = !showSettings">Settings</button>
  </div>
  <SettingsPanel v-if="showSettings" />
</template>
