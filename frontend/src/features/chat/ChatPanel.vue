<script setup lang="ts">
import { onMounted } from 'vue'
import { useChat } from './useChat'
import AnswerBox from '../overlay/AnswerBox.vue'
import TextInput from '../overlay/TextInput.vue'

const { messages, loading, error, load, send } = useChat()

onMounted(() => {
  void load()
})
</script>

<template>
  <div class="chat-panel">
    <div class="history">
      <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
        <span class="label">{{ m.role }}:</span>
        <span class="text">{{ m.text }}</span>
      </div>
    </div>
    <AnswerBox :answer="error ?? ''" :loading="loading" placeholder="Waiting" />
    <TextInput :disabled="loading" @submit="send" />
  </div>
</template>
