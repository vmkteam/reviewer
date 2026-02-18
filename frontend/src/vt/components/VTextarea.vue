<template>
  <textarea
    :value="modelValue"
    @input="onInput"
    :rows="rows"
    :placeholder="placeholder"
    v-bind="$attrs"
    class="w-full rounded-lg border border-edge-strong bg-surface text-fg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
  ></textarea>
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

defineProps<{
  modelValue?: string
  rows?: number
  placeholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLTextAreaElement).value)
}
</script>
