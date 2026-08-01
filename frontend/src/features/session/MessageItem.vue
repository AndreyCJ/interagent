<script setup lang="ts">
import type { Message } from '../../common/types/api.types'

defineProps<{
  message: Message
}>()
</script>

<template>
  <div class="message" :class="message.role">
    <span class="role">{{ message.role }}</span>
    <span class="text">{{ message.text }}</span>
  </div>
</template>
