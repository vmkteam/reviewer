<template>
  <input
    :value="modelValue"
    :type="type"
    :placeholder="placeholder"
    class="px-3 py-1.5 text-sm bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors placeholder:text-gray-300"
    v-bind="$attrs"
    @input="onInput"
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

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}
</script>
