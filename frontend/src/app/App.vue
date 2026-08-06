<script setup lang="ts">
import { onMounted } from 'vue'
import ChatPanel from '../features/chat/ChatPanel.vue'
import OverlayWindow from '../features/overlay/OverlayWindow.vue'
import { useOverlay } from '../features/overlay/useOverlay'
import { useAudio } from '../features/audio/useAudio'
import MicButton from '../features/audio/MicButton.vue'

const { mode, loadMode, toggleMode, show, hide } = useOverlay()
const { isListening, error, load, toggle, openSettings } = useAudio()

onMounted(() => {
  loadMode()
  load()
})
</script>

<template>
  <OverlayWindow :visible="true" theme="transparent">
    <div class="controls">
      <MicButton :listening="isListening" @toggle="toggle" />
      <span v-if="error" class="err">{{ error }}</span>
      <button v-if="error" @click="openSettings">Open Settings</button>
      <button @click="toggleMode">Mode: {{ mode }}</button>
      <button @click="show">Show</button>
      <button @click="hide">Hide</button>
    </div>
    <ChatPanel />
  </OverlayWindow>
</template>
