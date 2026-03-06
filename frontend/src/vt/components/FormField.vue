<template>
  <div class="mb-4">
    <div class="flex items-baseline gap-2 mb-1">
      <label class="text-sm font-medium text-fg-secondary">{{ label }}</label>
      <a v-if="fillable" @click="$emit('fill')" class="text-xs text-fg-subtle/60 border-b border-dashed border-fg-subtle/40 cursor-pointer hover:text-accent leading-none select-none">fill</a>
    </div>
    <slot />
    <p v-if="error" class="mt-1 text-sm text-danger">{{ error }}</p>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  label: string
  error?: string
  fillable?: boolean
}>()

defineEmits<{ fill: [] }>()
</script>
