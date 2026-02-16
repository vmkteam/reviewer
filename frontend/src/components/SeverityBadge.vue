<template>
  <span class="badge" :class="badgeClass">
    {{ severity }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{ severity: string }>()

const badgeClass = computed(() => ({
  critical: 'bg-red-50 text-red-700 ring-1 ring-red-200',
  high: 'bg-orange-50 text-orange-700 ring-1 ring-orange-200',
  medium: 'bg-amber-50 text-amber-700 ring-1 ring-amber-200',
  low: 'bg-sky-50 text-sky-700 ring-1 ring-sky-200',
}[props.severity] ?? 'bg-gray-50 text-gray-600 ring-1 ring-gray-200'))
</script>
