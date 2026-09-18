<script setup lang="ts">
defineProps<{
  listening: boolean
  busy?: boolean
}>()

defineEmits<{
  toggle: []
}>()
</script>

<template>
  <button
    class="mic-button"
    :class="{ active: listening }"
    :disabled="busy"
    @click="$emit('toggle')"
  >
    <span v-if="busy" class="mic-spinner" aria-hidden="true"></span>
    {{ busy ? 'Loading...' : listening ? 'Stop' : 'Listen' }}
  </button>
</template>
