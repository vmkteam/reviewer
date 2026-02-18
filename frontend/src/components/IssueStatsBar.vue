<template>
  <div class="flex items-center gap-1">
    <span v-if="stats.critical" class="badge bg-red-50 text-red-700 ring-1 ring-red-200 dark:bg-red-950 dark:text-red-300 dark:ring-red-800">
      C {{ stats.critical }}
    </span>
    <span v-if="stats.high" class="badge bg-orange-50 text-orange-700 ring-1 ring-orange-200 dark:bg-orange-950 dark:text-orange-300 dark:ring-orange-800">
      H {{ stats.high }}
    </span>
    <span v-if="stats.medium" class="badge bg-amber-50 text-amber-700 ring-1 ring-amber-200 dark:bg-amber-950 dark:text-amber-300 dark:ring-amber-800">
      M {{ stats.medium }}
    </span>
    <span v-if="stats.low" class="badge bg-sky-50 text-sky-700 ring-1 ring-sky-200 dark:bg-sky-950 dark:text-sky-300 dark:ring-sky-800">
      L {{ stats.low }}
    </span>
    <span v-if="stats.total === 0" class="text-xs text-fg-faint">&mdash;</span>
  </div>
</template>

<script setup lang="ts">
import type { IssueStats } from '../api/factory'

defineProps<{ stats: IssueStats }>()
</script>
