<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center" @keydown.esc="$emit('cancel')" tabindex="-1" ref="dialogRef">
      <div class="fixed inset-0 bg-overlay" @click="$emit('cancel')"></div>
      <div class="relative bg-surface rounded-xl shadow-xl max-w-sm w-full mx-4 p-6">
        <h3 class="text-lg font-semibold text-fg mb-2">{{ title }}</h3>
        <p class="text-sm text-fg-secondary mb-6">{{ message }}</p>
        <div class="flex justify-end gap-3">
          <VButton variant="secondary" @click="$emit('cancel')">Cancel</VButton>
          <VButton variant="danger" @click="$emit('confirm')">Delete</VButton>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, watch, nextTick } from 'vue'
import VButton from './VButton.vue'

const props = defineProps<{
  open: boolean
  title?: string
  message?: string
}>()

defineEmits<{
  confirm: []
  cancel: []
}>()

const dialogRef = ref<HTMLElement>()

watch(() => props.open, (v) => {
  if (v) nextTick(() => dialogRef.value?.focus())
})
</script>
