<template>
  <div>
    <h1 class="text-2xl font-bold text-gray-900 mb-6">Projects</h1>

    <div v-if="loading" class="flex justify-center py-16">
      <div class="spinner spinner-lg" />
    </div>

    <div v-else-if="error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>

    <div v-else-if="projects.length === 0" class="text-gray-400 text-center py-16 text-sm">No projects found.</div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
      <router-link
        v-for="p in projects"
        :key="p.projectId"
        :to="{ name: 'reviews', params: { id: p.projectId } }"
        class="block bg-white rounded-xl border border-gray-200 p-5 card-hover"
      >
        <div class="flex items-start justify-between mb-4">
          <h2 class="text-base font-semibold text-gray-900 leading-snug truncate">{{ p.title }}</h2>
          <span class="ml-3 flex-shrink-0 badge bg-gray-100 text-gray-600">
            {{ p.language }}
          </span>
        </div>

        <div class="text-sm text-gray-400 mb-4">
          {{ p.reviewCount }} review{{ p.reviewCount !== 1 ? 's' : '' }}
        </div>

        <div v-if="p.lastReview" class="flex items-center gap-2.5 text-sm pt-3 border-t border-gray-100">
          <TrafficLight :color="p.lastReview.trafficLight" />
          <span class="text-gray-600 truncate">{{ p.lastReview.author }}</span>
          <span class="text-gray-400 ml-auto text-xs">
            <TimeAgo :date="p.lastReview.createdAt" />
          </span>
        </div>
        <div v-else class="text-xs text-gray-300 pt-3 border-t border-gray-100">No reviews yet</div>
      </router-link>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import api, { type Project } from '../api/factory'
import TrafficLight from '../components/TrafficLight.vue'
import TimeAgo from '../components/TimeAgo.vue'
import { clearCrumbs } from '../utils/breadcrumbs'

const projects = ref<Project[]>([])
const loading = ref(true)
const error = ref('')

clearCrumbs()

onMounted(async () => {
  try {
    projects.value = await api.review.projects()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load projects'
  } finally {
    loading.value = false
  }
})
</script>
