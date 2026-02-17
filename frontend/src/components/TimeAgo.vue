<template>
  <span :title="formatDateTime(date)">{{ timeAgo(date) }}</span>
</template>

<script setup lang="ts">
import { useFormat } from '../composables/useFormat'

const { timeAgo, formatDateTime } = useFormat()

defineProps<{ date: string }>()
</script>
