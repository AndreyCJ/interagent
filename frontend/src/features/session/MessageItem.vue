<template>
  <div class="message" :class="message.role">
    <span class="role">{{ message.role }}</span>
    <span class="text">{{ message.text }}</span>
  </div>
</template>

<script setup lang="ts">
import type { Message } from '../../types'

defineProps<{
  message: Message
}>()
</script>
