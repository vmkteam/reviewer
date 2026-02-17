<template>
  <Teleport to="body">
    <div v-if="open" class="fixed inset-0 z-50 flex items-center justify-center">
      <div class="fixed inset-0 bg-black/40" @click="$emit('cancel')"></div>
      <div class="relative bg-white rounded-xl shadow-xl max-w-sm w-full mx-4 p-6">
        <h3 class="text-lg font-semibold text-gray-900 mb-2">{{ title }}</h3>
        <p class="text-sm text-gray-600 mb-6">{{ message }}</p>
        <div class="flex justify-end gap-3">
          <VButton variant="secondary" @click="$emit('cancel')">Cancel</VButton>
          <VButton variant="danger" @click="$emit('confirm')">Delete</VButton>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import VButton from './VButton.vue'

defineProps<{
  open: boolean
  title?: string
  message?: string
}>()

defineEmits<{
  confirm: []
  cancel: []
}>()
</script>
