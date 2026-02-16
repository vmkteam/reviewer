<template>
  <input
    :value="modelValue"
    :type="type"
    :placeholder="placeholder"
    class="px-3 py-1.5 text-sm bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors placeholder:text-gray-300"
    v-bind="$attrs"
    @input="$emit('update:modelValue', ($event.target as HTMLInputElement).value)"
  >
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

withDefaults(defineProps<{
  modelValue: string
  type?: string
  placeholder?: string
}>(), {
  type: 'text',
  placeholder: '',
})

defineEmits<{
  'update:modelValue': [value: string]
}>()
</script>
