<template>
  <div class="mb-4">
    <label class="block text-sm font-medium text-fg-secondary mb-1">{{ label }}</label>
    <slot />
    <p v-if="error" class="mt-1 text-sm text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  error?: string
}>()
</script>
