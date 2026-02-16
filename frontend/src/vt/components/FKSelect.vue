<template>
  <VSelect
    :model-value="modelValue"
    @update:model-value="$emit('update:modelValue', toValue($event))"
  >
    <option v-if="nullable" :value="undefined">-- None --</option>
    <option v-for="opt in options" :key="opt.id" :value="opt.id">{{ opt.title }}</option>
  </VSelect>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import VSelect from './VSelect.vue'

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

function toValue(val: unknown): number | null | undefined {
  if (val === undefined || val === null || val === '') return props.nullable ? null : undefined
  if (typeof val === 'number') return val
  return parseInt(String(val), 10)
}

onMounted(async () => {
  try {
    options.value = await props.loadFn()
  } catch {
    options.value = []
  }
})
</script>
