<template>
  <div class="flex items-center gap-1">
    <button
      class="px-2 py-1 text-xs rounded-md border transition-all cursor-pointer"
      :class="statusId === StatusValid
        ? 'bg-emerald-50 border-emerald-300 text-emerald-700 font-medium dark:bg-emerald-950 dark:border-emerald-700 dark:text-emerald-300'
        : 'border-edge text-fg-subtle hover:border-emerald-300 hover:text-emerald-600 hover:bg-emerald-50/50 dark:hover:border-emerald-700 dark:hover:text-emerald-400 dark:hover:bg-emerald-950/50'"
      title="Confirmed issue"
      @click="$emit('feedback', statusId === StatusValid ? StatusEnabled : StatusValid)"
    >Valid</button>
    <button
      class="px-2 py-1 text-xs rounded-md border transition-all cursor-pointer"
      :class="statusId === StatusFalsePositive
        ? 'bg-red-50 border-red-300 text-red-700 font-medium dark:bg-red-950 dark:border-red-700 dark:text-red-300'
        : 'border-edge text-fg-subtle hover:border-red-300 hover:text-red-600 hover:bg-red-50/50 dark:hover:border-red-700 dark:hover:text-red-400 dark:hover:bg-red-950/50'"
      title="False positive"
      @click="$emit('feedback', statusId === StatusFalsePositive ? StatusEnabled : StatusFalsePositive)"
    >FP</button>
    <button
      class="px-2 py-1 text-xs rounded-md border transition-all cursor-pointer"
      :class="statusId === StatusIgnored
        ? 'bg-amber-50 border-amber-300 text-amber-700 font-medium dark:bg-amber-950 dark:border-amber-700 dark:text-amber-300'
        : 'border-edge text-fg-subtle hover:border-amber-300 hover:text-amber-600 hover:bg-amber-50/50 dark:hover:border-amber-700 dark:hover:text-amber-400 dark:hover:bg-amber-950/50'"
      title="Intentionally ignored"
      @click="$emit('feedback', statusId === StatusIgnored ? StatusEnabled : StatusIgnored)"
    >Ignore</button>
  </div>
</template>

<script setup lang="ts">
import { StatusEnabled, StatusValid, StatusFalsePositive, StatusIgnored } from '../constants/status'

defineProps<{
  statusId: number
}>()

defineEmits<{
  feedback: [statusId: number]
}>()
</script>
