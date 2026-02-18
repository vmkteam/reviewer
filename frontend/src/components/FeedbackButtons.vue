<template>
  <div class="flex items-center gap-1">
    <button
      class="px-2 py-1 text-xs rounded-md border transition-all"
      :class="isFalsePositive === false
        ? 'bg-emerald-50 border-emerald-300 text-emerald-700 font-medium'
        : 'border-gray-200 text-gray-400 hover:border-emerald-300 hover:text-emerald-600 hover:bg-emerald-50/50'"
      title="Confirmed issue"
      @click="$emit('feedback', isFalsePositive === false ? null : false)"
    >Valid</button>
    <button
      class="px-2 py-1 text-xs rounded-md border transition-all"
      :class="isFalsePositive === true
        ? 'bg-red-50 border-red-300 text-red-700 font-medium'
        : 'border-gray-200 text-gray-400 hover:border-red-300 hover:text-red-600 hover:bg-red-50/50'"
      title="False positive"
      @click="$emit('feedback', isFalsePositive === true ? null : true)"
    >FP</button>
  </div>
</template>

<script setup lang="ts">
defineProps<{
  isFalsePositive: boolean | null | undefined
}>()

defineEmits<{
  feedback: [value: boolean | null]
}>()
</script>
