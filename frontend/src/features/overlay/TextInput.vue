<script setup lang="ts">
import { ref } from 'vue'

const props = withDefaults(
  defineProps<{
    disabled?: boolean
  }>(),
  { disabled: false },
)

const emit = defineEmits<{
  submit: [text: string]
}>()

const text = ref('')

function submit(): void {
  if (props.disabled) return
  const trimmed = text.value.trim()
  if (!trimmed) return
  emit('submit', trimmed)
  text.value = ''
}
</script>

<template>
  <div class="text-input">
    <input v-model="text" type="text" :disabled="disabled" @keyup.enter="submit" />
    <button :disabled="disabled" @click="submit">Send</button>
  </div>
</template>
