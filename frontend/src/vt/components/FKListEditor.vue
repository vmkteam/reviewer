<template>
  <div>
    <ul v-if="list.length" class="space-y-1 mb-2">
      <li
        v-for="(id, i) in list"
        :key="i"
        class="flex items-center gap-2 rounded-lg border border-edge bg-surface-alt px-3 py-1.5 text-sm"
      >
        <span class="w-5 text-xs tabular-nums text-fg-subtle">{{ i + 1 }}.</span>
        <span class="flex-1 text-fg">{{ titleOf(id) }}</span>
        <button
          type="button"
          @click="move(i, -1)"
          :disabled="i === 0"
          class="px-1 text-fg-subtle hover:text-accent disabled:opacity-30 disabled:hover:text-fg-subtle"
          title="Move up"
        >↑</button>
        <button
          type="button"
          @click="move(i, 1)"
          :disabled="i === list.length - 1"
          class="px-1 text-fg-subtle hover:text-accent disabled:opacity-30 disabled:hover:text-fg-subtle"
          title="Move down"
        >↓</button>
        <button
          type="button"
          @click="removeAt(i)"
          class="px-1 text-fg-subtle hover:text-danger"
          title="Remove"
        >✕</button>
      </li>
    </ul>

    <select
      :value="''"
      @change="onAdd"
      class="app-select cursor-pointer w-full rounded-lg border border-edge-strong bg-surface text-fg px-3 py-2 text-sm"
    >
      <option value="" disabled>{{ placeholder }}</option>
      <option v-for="opt in options" :key="opt.id" :value="opt.id">{{ opt.title }}</option>
    </select>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'

interface FKOption {
  id: number
  title: string
}

const props = withDefaults(defineProps<{
  modelValue?: number[]
  loadFn: () => Promise<FKOption[]>
  placeholder?: string
}>(), {
  placeholder: '+ Add…',
})

const emit = defineEmits<{
  'update:modelValue': [value: number[]]
}>()

const options = ref<FKOption[]>([])

// list is the current value as a guaranteed array (undefined → []). Mutations
// emit a fresh array so the parent's reactive binding always sees a new value.
const list = computed<number[]>(() => props.modelValue ?? [])

function titleOf(id: number): string {
  return options.value.find(o => o.id === id)?.title ?? `#${id}`
}

function onAdd(e: Event) {
  const select = e.target as HTMLSelectElement
  const id = parseInt(select.value, 10)
  select.value = '' // reset back to the placeholder
  if (!Number.isNaN(id)) {
    emit('update:modelValue', [...list.value, id]) // duplicates allowed (self-fusion)
  }
}

function removeAt(i: number) {
  const next = [...list.value]
  next.splice(i, 1)
  emit('update:modelValue', next)
}

function move(i: number, delta: number) {
  const j = i + delta
  if (j < 0 || j >= list.value.length) return
  const next = [...list.value]
  ;[next[i], next[j]] = [next[j], next[i]]
  emit('update:modelValue', next)
}

onMounted(async () => {
  try {
    options.value = await props.loadFn()
  } catch {
    options.value = []
  }
})
</script>
