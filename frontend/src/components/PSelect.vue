<template>
  <select
    :value="modelValue"
    class="px-3 py-1.5 text-sm bg-surface border border-edge rounded-lg focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-accent transition-colors"
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
