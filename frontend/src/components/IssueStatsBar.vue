<template>
  <div class="flex items-center gap-1">
    <span v-if="stats.critical" class="badge bg-red-50 text-red-700 ring-1 ring-red-200">
      C {{ stats.critical }}
    </span>
    <span v-if="stats.high" class="badge bg-orange-50 text-orange-700 ring-1 ring-orange-200">
      H {{ stats.high }}
    </span>
    <span v-if="stats.medium" class="badge bg-amber-50 text-amber-700 ring-1 ring-amber-200">
      M {{ stats.medium }}
    </span>
    <span v-if="stats.low" class="badge bg-sky-50 text-sky-700 ring-1 ring-sky-200">
      L {{ stats.low }}
    </span>
    <span v-if="stats.total === 0" class="text-xs text-gray-300">&mdash;</span>
  </div>
</template>

<script setup lang="ts">
import type { IssueStats } from '../api/factory'

defineProps<{ stats: IssueStats }>()
</script>
