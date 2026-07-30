<template>
  <div class="shortcut-editor">
    <div v-for="shortcut in shortcuts" :key="shortcut.id" class="shortcut">
      <span>{{ shortcut.label }}</span>
      <span>{{ shortcut.keys.join(' + ') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Shortcut } from '../../types'

defineProps<{
  shortcuts: Shortcut[]
}>()
</script>
