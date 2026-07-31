<script setup lang="ts">
defineProps<{
  answer: string
  loading: boolean
  placeholder?: string
}>()
</script>

<template>
  <div class="answer-box">
    <div v-if="loading" class="loading">...</div>
    <div v-else-if="answer" class="answer">
      {{ answer }}
    </div>
    <div v-else class="empty">
      {{ placeholder }}
    </div>
  </div>
</template>
