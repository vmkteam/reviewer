<template>
  <div class="relative">
    <input
      :value="modelValue"
      @input="onInput"
      @focus="open = true"
      @blur="open = false"
      @keydown="onKeydown"
      :placeholder="placeholder"
      type="text"
      autocomplete="off"
      class="w-full rounded-lg border border-edge-strong bg-surface text-fg px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-accent"
    />
    <ul
      v-if="open && filtered.length"
      class="absolute z-20 mt-1 w-full max-h-60 overflow-auto rounded-lg border border-edge-strong bg-surface shadow-lg py-1"
    >
      <li
        v-for="(opt, i) in filtered"
        :key="opt"
        @mousedown.prevent="select(opt)"
        @mouseenter="highlighted = i"
        :class="['px-3 py-2 text-sm cursor-pointer', i === highlighted ? 'bg-accent text-white' : 'text-fg']"
      >
        {{ opt }}
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'

// Free-text input with a themed suggestion dropdown — a styleable replacement for
// the native <datalist>, which the browser renders unthemed (light popup).
const props = defineProps<{
  modelValue?: string
  suggestions?: string[]
  placeholder?: string
}>()

const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const open = ref(false)
const highlighted = ref(-1)

// Substring filter on the current value; empty value shows the full list.
const filtered = computed(() => {
  const all = props.suggestions ?? []
  const q = (props.modelValue ?? '').toLowerCase().trim()
  return q ? all.filter((s) => s.toLowerCase().includes(q)) : all
})

// Reset the highlight whenever the visible list changes.
watch(filtered, () => { highlighted.value = -1 })

function onInput(e: Event) {
  emit('update:modelValue', (e.target as HTMLInputElement).value)
  open.value = true
}

function select(opt: string) {
  emit('update:modelValue', opt)
  open.value = false
}

function onKeydown(e: KeyboardEvent) {
  if (!open.value && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
    open.value = true
    return
  }
  if (!open.value) return
  const n = filtered.value.length
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    highlighted.value = n ? (highlighted.value + 1) % n : -1
  } else if (e.key === 'ArrowUp') {
    e.preventDefault()
    highlighted.value = n ? (highlighted.value - 1 + n) % n : -1
  } else if (e.key === 'Enter' && highlighted.value >= 0 && highlighted.value < n) {
    e.preventDefault()
    select(filtered.value[highlighted.value])
  } else if (e.key === 'Escape') {
    open.value = false
  }
}
</script>
