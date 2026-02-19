<template>
  <div class="bg-surface rounded-xl border border-edge overflow-x-auto shadow-sm">
    <table class="min-w-full">
      <thead>
        <tr class="border-b border-edge-light">
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider w-10"></th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider">Title</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider">Author</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider hidden lg:table-cell">Branch</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider">Reviews</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider">Issues</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-fg-subtle uppercase tracking-wider whitespace-nowrap">Date</th>
        </tr>
      </thead>
      <tbody>
        <tr
          v-for="r in reviews"
          :key="r.reviewId"
          class="border-b border-edge-light last:border-b-0 transition-colors row-hover"
          :class="isCurrent(r.reviewId)
            ? 'bg-accent-light/30 cursor-default'
            : 'hover:bg-accent-light/40 cursor-pointer'"
          @click="onRowClick(r.reviewId)"
        >
          <td class="px-4 py-3.5">
            <TrafficLight :color="r.trafficLight" />
          </td>
          <td class="px-4 py-3.5">
            <div class="text-sm font-medium" :class="!isCurrent(r.reviewId) && r.lastVersionReviewId ? 'text-fg-subtle' : 'text-fg'">{{ r.title }}</div>
            <div class="text-xs text-fg-faint mt-0.5">
              {{ r.externalId }}
              <router-link
                v-if="r.lastVersionReviewId && !currentReviewId"
                :to="{ name: 'review', params: { id: r.lastVersionReviewId } }"
                class="ml-1 text-amber-500 hover:text-amber-700 dark:text-amber-400 dark:hover:text-amber-300"
                @click.stop
                title="Go to latest version"
              >
                <svg class="w-3 h-3 inline" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"/></svg>
              </router-link>
            </div>
          </td>
          <td class="px-4 py-3.5 text-sm text-fg-secondary">{{ r.author }}</td>
          <td class="px-4 py-3.5 hidden lg:table-cell">
            <div class="flex items-center gap-1 text-xs">
              <span class="font-mono text-fg-muted truncate max-w-[180px]">{{ r.sourceBranch }}</span>
              <svg class="w-3 h-3 text-fg-faint flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
              <span class="font-mono text-fg-muted truncate max-w-[180px]">{{ r.targetBranch }}</span>
            </div>
          </td>
          <td class="px-4 py-3.5">
            <ReviewTypeDots :review-files="r.reviewFiles" />
          </td>
          <td class="px-4 py-3.5">
            <span class="text-sm tabular-nums" :class="totalIssues(r) > 0 ? 'text-fg-secondary font-medium' : 'text-fg-faint'">
              {{ totalIssues(r) || '\u2014' }}
            </span>
          </td>
          <td class="px-4 py-3.5 text-xs text-fg-subtle whitespace-nowrap">
            <TimeAgo :date="r.createdAt" />
          </td>
        </tr>
        <tr v-if="reviews.length === 0">
          <td colspan="7" class="px-4 py-12 text-center text-sm text-fg-subtle">No reviews found.</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import type { ReviewSummary } from '../api/factory'
import TrafficLight from './TrafficLight.vue'
import ReviewTypeDots from './ReviewTypeDots.vue'
import TimeAgo from './TimeAgo.vue'

const props = defineProps<{
  reviews: ReviewSummary[]
  currentReviewId?: number
}>()

const emit = defineEmits<{
  click: [reviewId: number]
}>()

function isCurrent(reviewId: number): boolean {
  return props.currentReviewId !== undefined && props.currentReviewId === reviewId
}

function totalIssues(r: ReviewSummary): number {
  return r.reviewFiles.reduce((sum, rf) => sum + rf.issueStats.total, 0)
}

function onRowClick(reviewId: number) {
  if (!isCurrent(reviewId)) {
    emit('click', reviewId)
  }
}
</script>
