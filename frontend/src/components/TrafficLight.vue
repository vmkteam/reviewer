<template>
  <span
    class="inline-block rounded-full flex-shrink-0"
    :class="[sizeClass, colorClass]"
    :title="color"
  />
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = withDefaults(defineProps<{
  color: string
  size?: 'sm' | 'md' | 'lg'
}>(), { size: 'sm' })

const sizeClass = computed(() => ({
  sm: 'w-2.5 h-2.5',
  md: 'w-3.5 h-3.5',
  lg: 'w-5 h-5',
}[props.size]))

const colorClass = computed(() => ({
  red: 'bg-red-500 shadow-[0_0_6px_rgba(239,68,68,0.4)] dark:bg-red-400 dark:shadow-[0_0_6px_rgba(248,113,113,0.4)]',
  yellow: 'bg-amber-400 shadow-[0_0_6px_rgba(251,191,36,0.4)] dark:bg-amber-300 dark:shadow-[0_0_6px_rgba(252,211,77,0.4)]',
  green: 'bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.4)] dark:bg-emerald-400 dark:shadow-[0_0_6px_rgba(52,211,153,0.4)]',
}[props.color] ?? 'bg-fg-faint'))
</script>
