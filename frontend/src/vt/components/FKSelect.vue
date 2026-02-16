<template>
  <select
    :value="modelValue"
    @change="$emit('update:modelValue', toValue(($event.target as HTMLSelectElement).value))"
    class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
  >
    <option v-if="nullable" :value="undefined">-- None --</option>
    <option v-for="opt in options" :key="opt.id" :value="opt.id">{{ opt.title }}</option>
  </select>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'

interface FKOption {
  id: number
  title: string
}

const props = defineProps<{
  modelValue?: number | null
  loadFn: () => Promise<{ id: number; title: string }[]>
  nullable?: boolean
}>()

defineEmits<{
  'update:modelValue': [value: number | null | undefined]
}>()

const options = ref<FKOption[]>([])

function toValue(val: string): number | null | undefined {
  if (val === '' || val === 'undefined') return props.nullable ? null : undefined
  return parseInt(val, 10)
}

onMounted(async () => {
  try {
    options.value = await props.loadFn()
  } catch {
    options.value = []
  }
})
</script>
