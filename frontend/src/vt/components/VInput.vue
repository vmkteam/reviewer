<template>
  <input
    :value="modelValue"
    @input="onInput"
    :type="type"
    :placeholder="placeholder"
    :readonly="readonly"
    v-bind="$attrs"
    class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
  />
</template>

<script setup lang="ts">
defineOptions({ inheritAttrs: false })

defineProps<{
  modelValue?: unknown
  type?: string
  placeholder?: string
  readonly?: boolean
}>()

const emit = defineEmits<{
  'update:modelValue': [value: string]
}>()

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
}
</script>
