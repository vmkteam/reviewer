<template>
  <textarea
    :value="modelValue"
    :rows="rows"
    :placeholder="placeholder"
    class="flex-1 px-3 py-2 text-sm bg-surface border border-edge rounded-lg focus:outline-none focus:ring-2 focus:ring-accent/20 focus:border-accent transition-colors resize-none placeholder:text-fg-faint"
    v-bind="$attrs"
    @input="onInput"
  />
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

withDefaults(defineProps<{
  modelValue: string
  rows?: number
  placeholder?: string
}>(), {
  rows: 2,
  placeholder: '',
})

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
}
</script>
