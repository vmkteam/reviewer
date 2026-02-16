<template>
  <span
    class="badge"
    :class="statusId === 1 ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'"
  >{{ statusId === 1 ? 'Enabled' : 'Disabled' }}</span>
</template>

<script setup lang="ts">
defineProps<{
  statusId?: number
}>()
</script>
