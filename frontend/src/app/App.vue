<script setup lang="ts">
import { onMounted } from 'vue'
import ChatPanel from '../features/chat/ChatPanel.vue'
import OverlayWindow from '../features/overlay/OverlayWindow.vue'
import { useOverlay } from '../features/overlay/useOverlay'

const { mode, loadMode, toggleMode, show, hide } = useOverlay()

onMounted(() => {
  loadMode()
})
</script>

<template>
  <OverlayWindow :visible="true" theme="transparent">
    <div class="controls">
      <button @click="toggleMode">Mode: {{ mode }}</button>
      <button @click="show">Show</button>
      <button @click="hide">Hide</button>
    </div>
    <ChatPanel />
  </OverlayWindow>
</template>
