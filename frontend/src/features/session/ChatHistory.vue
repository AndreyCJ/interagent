<script setup lang="ts">
import type { Message } from '../../types'
import MessageItem from './MessageItem.vue'

defineProps<{
  messages: Message[]
}>()
</script>

<template>
  <div class="chat-history">
    <MessageItem v-for="msg in messages" :key="msg.timestamp" :message="msg" />
  </div>
</template>
