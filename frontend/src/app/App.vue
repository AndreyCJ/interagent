<script setup lang="ts">
import { onMounted, ref } from 'vue'
import MicButton from '../features/audio/MicButton.vue'
import { useAudio } from '../features/audio/useAudio'
import ChatPanel from '../features/chat/ChatPanel.vue'
import OverlayWindow from '../features/overlay/OverlayWindow.vue'
import { useOverlay } from '../features/overlay/useOverlay'
import SettingsPanel from '../features/settings/SettingsPanel.vue'

const { mode, loadMode: loadOverlayMode, toggleMode: toggleOverlayMode, show, hide } = useOverlay()
const { isListening, error, load: loadAudio, toggle: toggleAudio, openSettings } = useAudio()
const showSettings = ref(false)

onMounted(() => {
  loadOverlayMode()
  loadAudio()
})
</script>

<template>
  <OverlayWindow :visible="true" theme="transparent">
    <div class="controls">
      <MicButton :listening="isListening" @toggle="toggleAudio" />
      <span v-if="error" class="err">{{ error }}</span>
      <button v-if="error" @click="openSettings">Open Settings</button>
      <button @click="toggleOverlayMode">Mode: {{ mode }}</button>
      <button @click="show">Show</button>
      <button @click="hide">Hide</button>
      <button @click="showSettings = !showSettings">Settings</button>
    </div>
    <SettingsPanel v-if="showSettings" />
    <ChatPanel />
  </OverlayWindow>
</template>
