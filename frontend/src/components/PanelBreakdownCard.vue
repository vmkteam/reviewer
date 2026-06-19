<template>
  <div class="bg-surface rounded-xl border border-edge p-4 sm:p-6 mb-6 shadow-sm">
    <div class="flex items-center gap-2 mb-1">
      <h2 class="text-sm font-semibold text-fg">Panel breakdown</h2>
      <span class="inline-flex items-center px-2 py-0.5 text-[10px] font-medium rounded-full bg-accent-light text-accent">fusion</span>
    </div>
    <p class="text-xs text-fg-subtle mb-4">
      Synthesized from {{ review.members.length }} panel {{ review.members.length === 1 ? 'member' : 'members' }}.
      Each member was reviewed independently; this fusion is the judge's merged result.
    </p>

    <ul class="divide-y divide-edge-light border-y border-edge-light">
      <li v-for="m in review.members" :key="m.reviewId">
        <router-link
          :to="{ name: 'review', params: { id: m.reviewId } }"
          class="flex items-center gap-3 py-2.5 hover:bg-surface-alt/60 transition-colors -mx-2 px-2 rounded-lg"
        >
          <TrafficLight :color="m.trafficLight" />
          <span class="flex-1 min-w-0">
            <span class="block text-sm text-fg truncate">{{ m.model || m.title }}</span>
            <span class="block text-xs text-fg-subtle">{{ issueCount(m) }} {{ issueCount(m) === 1 ? 'issue' : 'issues' }}</span>
          </span>
          <span class="text-sm text-fg-secondary tabular-nums">{{ formatCost(m.costUsd) }}</span>
          <svg class="w-4 h-4 text-fg-faint flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
        </router-link>
      </li>
    </ul>

    <div class="grid grid-cols-3 gap-x-6 gap-y-2 mt-4 text-sm">
      <div>
        <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-0.5">Panel</div>
        <div class="text-fg-secondary tabular-nums">{{ formatCost(review.panelCostUsd) }}</div>
      </div>
      <div>
        <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-0.5">Judge</div>
        <div class="text-fg-secondary tabular-nums">{{ formatCost(review.modelInfo.costUsd) }}</div>
      </div>
      <div>
        <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-0.5">Total</div>
        <div class="text-fg font-medium tabular-nums">{{ formatCost(review.panelCostUsd + review.modelInfo.costUsd) }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import type { Review, PanelMember } from '../api/factory'
import TrafficLight from './TrafficLight.vue'
import { useFormat } from '../composables/useFormat'

defineProps<{ review: Review }>()

const { formatCost } = useFormat()

// A member's total issue count, summed across its review-type files.
function issueCount(m: PanelMember): number {
  return m.reviewFiles.reduce((sum, rf) => sum + (rf.issueStats?.total ?? 0), 0)
}
</script>
