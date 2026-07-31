<script setup lang="ts">
import type { AgentConfig } from '../../types'

defineProps<{
  agents: AgentConfig[]
}>()
</script>

<template>
  <div class="agent-manager">
    <div v-for="agent in agents" :key="agent.id" class="agent">
      {{ agent.name }}
    </div>
  </div>
</template>
