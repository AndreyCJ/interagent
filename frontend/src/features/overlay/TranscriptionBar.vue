<template>
  <div class="transcription-bar">
    <span v-if="text" class="text">{{ text }}</span>
    <span v-else class="placeholder">{{ placeholder }}</span>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  text: string
  placeholder?: string
}>()
</script>
