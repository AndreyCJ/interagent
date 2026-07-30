<template>
  <div class="screenshot-tools">
    <button @click="$emit('captureFull')">Full Screen</button>
    <button @click="$emit('captureRegion')">Region</button>
  </div>
</template>

<script setup lang="ts">
defineEmits<{
  captureFull: []
  captureRegion: []
}>()
</script>
