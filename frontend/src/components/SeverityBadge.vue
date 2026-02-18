<template>
  <span class="badge" :class="badgeClass">
    {{ severity }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ severity: string }>()

const badgeClass = computed(() => ({
  critical: 'bg-red-50 text-red-700 ring-1 ring-red-200 dark:bg-red-950 dark:text-red-300 dark:ring-red-800',
  high: 'bg-orange-50 text-orange-700 ring-1 ring-orange-200 dark:bg-orange-950 dark:text-orange-300 dark:ring-orange-800',
  medium: 'bg-amber-50 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:ring-amber-800',
  low: 'bg-sky-50 text-sky-700 ring-1 ring-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:ring-sky-800',
}[props.severity] ?? 'bg-edge-light text-fg-secondary ring-1 ring-edge'))
</script>
