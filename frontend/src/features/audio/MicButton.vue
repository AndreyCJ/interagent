<script setup lang="ts">
defineProps<{
  listening: boolean
}>()

defineEmits<{
  toggle: []
}>()
</script>

<template>
  <button class="mic-button" :class="{ active: listening }" @click="$emit('toggle')">
    {{ listening ? 'Stop' : 'Listen' }}
  </button>
</template>
