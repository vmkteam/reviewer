<template>
  <select @change="onChange" class="app-select px-4 py-2 text-sm font-medium text-amber-700 dark:text-amber-300 border border-amber-300 dark:border-amber-700 rounded-lg hover:bg-amber-50 dark:hover:bg-amber-950 transition-colors bg-surface cursor-pointer">
    <option value="" disabled selected>Fill Example</option>
    <option v-for="(_, key) in presets" :key="key" :value="key">{{ key }}</option>
  </select>
</template>

<script setup lang="ts">
defineProps<{ presets: Record<string, any> }>()
const emit = defineEmits<{ select: [key: string] }>()

function onChange(event: Event) {
  const select = event.target as HTMLSelectElement
  if (select.value) {
    emit('select', select.value)
    select.value = ''
  }
}
</script>
