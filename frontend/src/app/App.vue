<script setup lang="ts">
import ChatPanel from '../features/chat/ChatPanel.vue'
import ControlsBar from '../features/overlay/ControlsBar.vue'
import OverlayWindow from '../features/overlay/OverlayWindow.vue'
</script>

<template>
  <OverlayWindow :visible="true" theme="transparent">
    <ControlsBar />
    <ChatPanel />
  </OverlayWindow>
</template>
