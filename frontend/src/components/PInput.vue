<template>
  <input
    :value="modelValue"
    :type="type"
    :placeholder="placeholder"
    class="px-3 py-1.5 text-sm bg-surface border border-edge rounded-lg focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-accent transition-colors placeholder:text-fg-faint"
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
