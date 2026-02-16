<template>
  <select
    :value="modelValue"
    class="px-3 py-1.5 text-sm bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors"
    v-bind="$attrs"
    @change="onChange"
  >
    <slot />
  </select>
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

defineProps<{
  modelValue: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function onChange(e: Event) {
  emit('update:modelValue', (e.target as HTMLSelectElement).value)
}
</script>
