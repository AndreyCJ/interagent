<script setup lang="ts">
import { onMounted, nextTick, watch } from 'vue'
import { useChat } from './useChat'
import { useAudio } from '../audio/useAudio'
import TextInput from '../overlay/TextInput.vue'

const { messages, loading, error, load, send } = useChat()
const { isListening } = useAudio()

function scrollToBottom() {
  nextTick(() => {
    const el = document.querySelector('.history')
    if (el) el.scrollTop = el.scrollHeight
  })
}

watch(messages, scrollToBottom, { deep: true })

onMounted(() => {
  void load()
  scrollToBottom()
})
</script>

<template>
  <div class="chat-panel">
    <div v-if="isListening" class="status-bar">
      <span class="listening-indicator">● Listening...</span>
    </div>
    <div class="history">
      <div v-for="(m, i) in messages" :key="i" class="msg" :class="m.role">
        <span class="label">{{ m.role }}:</span>
        <span class="text" :class="{ streaming: m.streaming }">{{ m.text }}</span>
      </div>
      <div
        v-if="
          loading &&
          (!messages.length ||
            messages[messages.length - 1].role !== 'assistant' ||
            !messages[messages.length - 1].streaming)
        "
        class="msg assistant"
      >
        <span class="label">assistant:</span>
        <span class="text streaming">...</span>
      </div>
    </div>
    <div v-if="error" class="error">{{ error }}</div>
    <TextInput :disabled="loading" @submit="send" />
  </div>
</template>

<style scoped>
.chat-panel {
  display: flex;
  flex-direction: column;
  height: 100%;
}
.status-bar {
  padding: 4px 8px;
  font-size: 12px;
  color: #4ade80;
}
.listening-indicator {
  animation: pulse 1.5s infinite;
}
@keyframes pulse {
  0%,
  100% {
    opacity: 1;
  }
  50% {
    opacity: 0.4;
  }
}
.history {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
}
.msg {
  margin-bottom: 6px;
  line-height: 1.4;
}
.msg .label {
  font-weight: 600;
  margin-right: 6px;
}
.msg.interviewer .label {
  color: #facc15;
}
.msg.user .label {
  color: #60a5fa;
}
.msg.assistant .label {
  color: #4ade80;
}
.msg .streaming {
  opacity: 0.7;
}
.error {
  padding: 4px 8px;
  color: #ef4444;
  font-size: 12px;
}
</style>
