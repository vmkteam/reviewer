<template>
  <div>
    <h1 class="text-2xl font-bold text-gray-900 mb-4">Projects</h1>

    <div v-if="projects.length > 5" class="mb-6 flex items-center gap-3">
      <PInput
        v-model="filterText"
        placeholder="Filter by name..."
        class="w-full sm:w-64"
      />
      <PSelect v-model="sortKey" class="ml-auto">
        <option value="title_asc">Title A→Z</option>
        <option value="title_desc">Title Z→A</option>
        <option value="review_desc">Last review ↓</option>
        <option value="review_asc">Last review ↑</option>
      </PSelect>
    </div>

    <div v-if="loading" class="flex justify-center py-16">
      <div class="spinner spinner-lg" />
    </div>

    <ErrorAlert v-else-if="error">{{ error }}</ErrorAlert>

    <div v-else-if="filteredProjects.length === 0" class="text-gray-400 text-center py-16 text-sm">No projects found.</div>

    <div v-else class="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5">
      <router-link
        v-for="p in filteredProjects"
        :key="p.projectId"
        :to="{ name: 'reviews', params: { id: p.projectId } }"
        class="block bg-white rounded-xl border border-gray-200 p-5 card-hover"
      >
        <div class="flex items-start justify-between mb-4">
          <h2 class="text-base font-semibold text-gray-900 leading-snug truncate">{{ p.title }}</h2>
          <InfoBadge class="ml-3 flex-shrink-0">{{ p.language }}</InfoBadge>
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
import { ref, computed, onMounted } from 'vue'
import api, { type Project } from '../api/factory'
import TrafficLight from '../components/TrafficLight.vue'
import TimeAgo from '../components/TimeAgo.vue'
import PInput from '../components/PInput.vue'
import PSelect from '../components/PSelect.vue'
import InfoBadge from '../components/InfoBadge.vue'
import ErrorAlert from '../components/ErrorAlert.vue'
import { useBreadcrumbs } from '../composables/useBreadcrumbs'

const { clear: clearCrumbs } = useBreadcrumbs()

const projects = ref<Project[]>([])
const loading = ref(true)
const error = ref('')
const filterText = ref('')
const sortKey = ref('title_asc')

const filteredProjects = computed(() => {
  const q = filterText.value.toLowerCase()
  const list = q ? projects.value.filter(p => p.title.toLowerCase().includes(q)) : [...projects.value]

  const key = sortKey.value
  list.sort((a, b) => {
    if (key === 'title_asc') return a.title.localeCompare(b.title)
    if (key === 'title_desc') return b.title.localeCompare(a.title)
    const da = a.lastReview?.createdAt ?? ''
    const db = b.lastReview?.createdAt ?? ''
    if (key === 'review_desc') return db.localeCompare(da)
    return da.localeCompare(db)
  })

  return list
})

clearCrumbs()
document.title = 'Projects — reviewer'

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
