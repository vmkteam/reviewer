<template>
  <div v-if="totalPages > 1" class="flex flex-col sm:flex-row items-center justify-between mt-4 gap-3 text-sm">
    <div class="text-fg-muted">
      Total: {{ total }}
    </div>
    <div class="flex items-center gap-1">
      <button
        :disabled="page <= 1"
        @click="$emit('update:page', page - 1)"
        class="px-3 py-1.5 rounded border border-edge-strong text-fg-secondary hover:bg-surface-alt disabled:opacity-40 disabled:cursor-not-allowed"
      >Prev</button>
      <button
        v-for="p in visiblePages"
        :key="p"
        @click="$emit('update:page', p)"
        class="px-3 py-1.5 rounded border hidden sm:inline-flex"
        :class="p === page ? 'bg-accent-light border-accent text-accent font-medium' : 'border-edge-strong text-fg-secondary hover:bg-surface-alt'"
      >{{ p }}</button>
      <span class="px-2 text-fg-muted sm:hidden">{{ page }} / {{ totalPages }}</span>
      <button
        :disabled="page >= totalPages"
        @click="$emit('update:page', page + 1)"
        class="px-3 py-1.5 rounded border border-edge-strong text-fg-secondary hover:bg-surface-alt disabled:opacity-40 disabled:cursor-not-allowed"
      >Next</button>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  page: number
  pageSize: number
  total: number
}>()

defineEmits<{
  'update:page': [page: number]
}>()

const totalPages = computed(() => Math.max(1, Math.ceil(props.total / props.pageSize)))

const visiblePages = computed(() => {
  const pages: number[] = []
  const tp = totalPages.value
  const cp = props.page
  let start = Math.max(1, cp - 2)
  let end = Math.min(tp, cp + 2)
  if (end - start < 4) {
    if (start === 1) end = Math.min(tp, start + 4)
    else start = Math.max(1, end - 4)
  }
  for (let i = start; i <= end; i++) pages.push(i)
  return pages
})
</script>
