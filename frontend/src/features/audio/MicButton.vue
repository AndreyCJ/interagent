<template>
  <button class="mic-button" :class="{ active: listening }" @click="$emit('toggle')">
    {{ listening ? 'Stop' : 'Mic' }}
  </button>
</template>

<script setup lang="ts">
defineProps<{
  listening: boolean
}>()

defineEmits<{
  toggle: []
}>()
</script>
