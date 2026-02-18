<template>
  <div class="flex items-center gap-0.5">
    <span
      v-for="rt in ordered"
      :key="rt.type"
      class="inline-flex items-center justify-center w-6 h-6 rounded text-[10px] font-semibold leading-none"
      :class="rt.cssClass"
      :title="`${rt.fullName}: ${rt.color}`"
    >
      {{ rt.label }}
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import type { ReviewFileSummary } from '../api/factory'
import { useFormat } from '../composables/useFormat'

const { reviewTypeLabel, reviewTypeFullName } = useFormat()

const props = defineProps<{ reviewFiles: ReviewFileSummary[] }>()

const typeOrder = ['architecture', 'code', 'security', 'tests']

const colorClasses: Record<string, string> = {
  red: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
  yellow: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
  green: 'bg-emerald-100 text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300',
}

const ordered = computed(() => {
  const byType = new Map(props.reviewFiles.map(f => [f.reviewType, f]))
  const result = []
  for (const t of typeOrder) {
    const rf = byType.get(t)
    const color = rf?.trafficLight
    if (color && color !== 'none') {
      result.push({
        type: t,
        label: reviewTypeLabel(t),
        fullName: reviewTypeFullName(t),
        color,
        cssClass: colorClasses[color] ?? 'bg-edge-light text-fg-muted',
      })
    }
  }
  return result
})
</script>
