<template>
  <div class="overlay" :class="themeClass">
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  visible: boolean
  theme?: 'dark' | 'light' | 'transparent'
}>()

const themeClass = computed(() => `theme-${props.theme ?? 'dark'}`)
</script>
