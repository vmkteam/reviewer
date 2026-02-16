<template>
  <div class="flex items-center gap-0.5">
    <span
      v-for="rt in ordered"
      :key="rt.type"
      class="inline-flex items-center justify-center w-6 h-6 rounded text-[10px] font-semibold leading-none"
      :class="dotClass(rt.color)"
      :title="`${rt.fullName}: ${rt.color}`"
    >
      {{ rt.label }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ReviewFileSummary } from '../api/factory'
import { reviewTypeLabel, reviewTypeFullName } from '../utils/format'

const props = defineProps<{ reviewFiles: ReviewFileSummary[] }>()

const typeOrder = ['architecture', 'code', 'security', 'tests']

const ordered = computed(() =>
  typeOrder
    .map(t => {
      const rf = props.reviewFiles.find(f => f.reviewType === t)
      return {
        type: t,
        label: reviewTypeLabel(t),
        fullName: reviewTypeFullName(t),
        color: rf?.trafficLight ?? 'none',
      }
    })
    .filter(r => r.color !== 'none')
)

function dotClass(color: string): string {
  return {
    red: 'bg-red-100 text-red-700',
    yellow: 'bg-amber-100 text-amber-700',
    green: 'bg-emerald-100 text-emerald-700',
  }[color] ?? 'bg-gray-100 text-gray-500'
}
</script>
