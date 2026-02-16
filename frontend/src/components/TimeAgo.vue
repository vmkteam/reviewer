<template>
  <span :title="formatDateTime(date)">{{ timeAgo(date) }}</span>
</template>

<script setup lang="ts">
import { timeAgo, formatDateTime } from '../utils/format'

defineProps<{ date: string }>()
</script>
