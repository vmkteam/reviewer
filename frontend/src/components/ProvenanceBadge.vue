<template>
  <span v-if="sources && sources.length" class="inline-flex flex-wrap items-center gap-1 align-middle">
    <span class="text-fg-subtle">Sources:</span>
    <span
      v-for="(src, i) in sources"
      :key="i"
      class="badge ring-1"
      :class="src === 'judge'
        ? 'bg-purple-50 text-purple-700 ring-purple-200 dark:bg-purple-950 dark:text-purple-300 dark:ring-purple-800'
        : 'bg-sky-50 text-sky-700 ring-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:ring-sky-800'"
      :title="src === 'judge' ? 'Verified by the judge (net-new finding)' : 'Flagged by ' + src"
    >{{ src }}</span>
    <span
      v-if="sources.length > 1"
      class="badge bg-edge-light text-fg-secondary ring-1 ring-edge"
      :title="sources.length + ' models agreed'"
    >✓{{ sources.length }}</span>
  </span>
</template>

<script setup lang="ts">
// ProvenanceBadge shows a fusion issue's provenance — the panel model labels that
// flagged it (or "judge" for a verified net-new), plus an agreement count when
// more than one model concurred. Renders nothing for single/member issues.
defineProps<{ sources?: string[] }>()
</script>
