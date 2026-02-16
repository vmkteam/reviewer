<template>
  <select
    ref="selectEl"
    @change="onChange"
    v-bind="$attrs"
    class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm"
  >
    <slot />
  </select>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, nextTick } from 'vue'

defineOptions({ inheritAttrs: false })

const props = defineProps<{
  modelValue?: unknown
}>()

const emit = defineEmits<{
  'update:modelValue': [value: unknown]
}>()

const selectEl = ref<HTMLSelectElement>()

function syncSelected() {
  const select = selectEl.value
  if (!select) return
  const options = Array.from(select.options)
  const idx = options.findIndex(opt => {
    const val = '_value' in opt ? (opt as any)._value : opt.value
    return val === props.modelValue
  })
  if (idx >= 0) select.selectedIndex = idx
}

function onChange(e: Event) {
  const select = e.target as HTMLSelectElement
  const option = select.selectedOptions[0] as any
  emit('update:modelValue', '_value' in option ? option._value : select.value)
}

onMounted(syncSelected)
watch(() => props.modelValue, () => nextTick(syncSelected))
</script>
