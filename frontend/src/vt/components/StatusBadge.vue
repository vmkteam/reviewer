<template>
  <span
    class="badge"
    :class="statusId === 1 ? 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300' : 'bg-edge-light text-fg-secondary'"
  >{{ statusId === 1 ? 'Enabled' : 'Disabled' }}</span>
</template>

<script setup lang="ts">
defineProps<{
  statusId?: number
}>()
</script>
