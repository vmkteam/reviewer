<template>
  <div>
    <!-- Back + Header -->
    <div class="mb-6">
      <router-link to="/" class="inline-flex items-center gap-1 text-sm text-fg-subtle hover:text-accent transition-colors mb-3">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 19l-7-7 7-7"/></svg>
        Projects
      </router-link>
      <div v-if="project" class="flex items-center gap-3 overflow-x-auto scrollbar-hide">
        <h1 class="text-2xl font-bold text-fg whitespace-nowrap">{{ project.title }}</h1>
        <InfoBadge>{{ project.language }}</InfoBadge>
        <div class="flex gap-1 ml-auto shrink-0">
          <ExternalLink v-if="project.taskTrackerURL?.trim()" :href="project.taskTrackerURL">Tracker</ExternalLink>
          <ExternalLink v-if="project.vcsURL" :href="project.vcsURL">VCS</ExternalLink>
        </div>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex border-b border-edge mb-5">
      <button
        @click="activeTab = 'reviews'"
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors"
        :class="activeTab === 'reviews'
          ? 'border-accent text-accent'
          : 'border-transparent text-fg-subtle hover:text-fg-secondary'"
      >
        Reviews
        <span v-if="totalCount !== null" class="ml-1 text-xs" :class="activeTab === 'reviews' ? 'text-accent/70' : 'text-fg-faint'">({{ totalCount }})</span>
      </button>
      <button
        @click="switchToRisks"
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors"
        :class="activeTab === 'risks'
          ? 'border-accent text-accent'
          : 'border-transparent text-fg-subtle hover:text-fg-secondary'"
      >
        Accepted Risks
        <span v-if="risksCount !== null" class="ml-1 text-xs" :class="activeTab === 'risks' ? 'text-accent/70' : 'text-fg-faint'">({{ risksCount }})</span>
      </button>
    </div>

    <!-- Reviews tab -->
    <div v-show="activeTab === 'reviews'">
      <!-- Filters -->
      <div class="flex flex-wrap items-center gap-3 mb-5">
        <PInput
          v-model="filters.title"
          placeholder="Filter by title..."
          class="w-full sm:w-44"
          @input="onFilterChange"
        />
        <PInput
          v-model="filters.author"
          placeholder="Filter by author..."
          class="w-full sm:w-44"
          @input="onFilterChange"
        />
        <PSelect
          v-model="filters.trafficLight"
          @change="onFilterChange"
        >
          <option value="">All statuses</option>
          <option value="green">Green</option>
          <option value="yellow">Yellow</option>
          <option value="red">Red</option>
        </PSelect>
      </div>

      <!-- Loading -->
      <div v-if="loadingInitial" class="flex justify-center py-16">
        <div class="spinner spinner-lg" />
      </div>

      <!-- Error -->
      <ErrorAlert v-else-if="error">{{ error }}</ErrorAlert>

      <!-- Table -->
      <div v-else>
        <InfiniteScroll :loading="loadingMore" :has-more="hasMore" @load-more="loadMore">
          <ReviewsTable :reviews="reviews" @click="goToReview" />
        </InfiniteScroll>
      </div>
    </div>

    <!-- Accepted Risks tab -->
    <div v-if="activeTab === 'risks'">
      <!-- Loading risks -->
      <div v-if="risksLoading && risks.length === 0" class="flex justify-center py-12">
        <div class="spinner" />
      </div>

      <!-- Risks error -->
      <ErrorAlert v-else-if="risksError">{{ risksError }}</ErrorAlert>

      <!-- Risks table -->
      <div v-else>
        <InfiniteScroll :loading="risksLoadingMore" :has-more="risksHasMore" @load-more="loadMoreRisks">
          <IssuesTable
            :issues="risks"
            :project="project"
            :sortable="false"
            :show-review-type="false"
            :show-local-id="false"
            :show-copy-link="true"
            :copied-issue-id="copiedIssueId"
            empty-text="No accepted risks found."
            v-model:expanded-id="expandedIssueId"
            @feedback="setRiskFeedback"
            @copy-link="copyIssueLink"
          />
        </InfiniteScroll>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import api, { type ReviewSummary, type Project, type ReviewFilters, type Issue } from '../api/factory'
import ReviewsTable from '../components/ReviewsTable.vue'
import InfiniteScroll from '../components/InfiniteScroll.vue'
import PInput from '../components/PInput.vue'
import PSelect from '../components/PSelect.vue'
import InfoBadge from '../components/InfoBadge.vue'
import ErrorAlert from '../components/ErrorAlert.vue'
import ExternalLink from '../components/ExternalLink.vue'
import IssuesTable from '../components/IssuesTable.vue'
import { useBreadcrumbs } from '../composables/useBreadcrumbs'

const { setProject: setProjectCrumb } = useBreadcrumbs()

const props = defineProps<{ id: string }>()
const router = useRouter()
const route = useRoute()

const projectId = parseInt(props.id, 10)
const project = ref<Project | null>(null)
const reviews = ref<ReviewSummary[]>([])
const totalCount = ref<number | null>(null)
const loadingInitial = ref(true)
const loadingMore = ref(false)
const hasMore = ref(true)
const error = ref('')
const activeTab = ref<'reviews' | 'risks'>('reviews')

const filters = reactive<{ title: string; author: string; trafficLight: string }>({
  title: typeof route.query.title === 'string' ? route.query.title : '',
  author: typeof route.query.author === 'string' ? route.query.author : '',
  trafficLight: '',
})

let filterTimeout: ReturnType<typeof setTimeout> | null = null

// Accepted Risks state
const risks = ref<Issue[]>([])
const risksCount = ref<number | null>(null)
const risksLoading = ref(false)
const risksLoadingMore = ref(false)
const risksHasMore = ref(true)
const risksError = ref('')
const risksLoaded = ref(false)
const expandedIssueId = ref<number | null>(null)
const copiedIssueId = ref<number | null>(null)

const risksFilters = { isFalsePositive: true }

function copyIssueLink(issueId: number) {
  const issue = risks.value.find(i => i.issueId === issueId)
  if (!issue) return
  const route = router.resolve({ name: 'review', params: { id: issue.reviewId } })
  const url = window.location.origin + route.href + '#issues-' + issueId
  navigator.clipboard.writeText(url)
  copiedIssueId.value = issueId
  setTimeout(() => {
    if (copiedIssueId.value === issueId) copiedIssueId.value = null
  }, 1500)
}

function buildFilters(): ReviewFilters | undefined {
  const f: ReviewFilters = {}
  if (filters.title) f.title = filters.title
  if (filters.author) f.author = filters.author
  if (filters.trafficLight) f.trafficLight = filters.trafficLight
  if (!f.title && !f.author && !f.trafficLight) return undefined
  return f
}

async function loadInitial() {
  loadingInitial.value = true
  error.value = ''
  reviews.value = []
  hasMore.value = true
  try {
    const [projectData, items, count] = await Promise.all([
      api.review.projectByID({ projectId }),
      api.review.get({ projectId, filters: buildFilters() }),
      api.review.count({ projectId, filters: buildFilters() }),
    ])
    project.value = projectData ?? null
    if (project.value) {
      setProjectCrumb(project.value.projectId, project.value.title)
      document.title = `${project.value.title} — reviewer`
    }
    reviews.value = items
    totalCount.value = count
    hasMore.value = items.length >= 50
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load reviews'
  } finally {
    loadingInitial.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value) return
  const lastId = reviews.value[reviews.value.length - 1]?.reviewId
  if (!lastId) return

  loadingMore.value = true
  try {
    const items = await api.review.get({ projectId, filters: buildFilters(), fromReviewId: lastId })
    reviews.value.push(...items)
    hasMore.value = items.length >= 50
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load more reviews'
  } finally {
    loadingMore.value = false
  }
}

function onFilterChange() {
  if (filterTimeout) clearTimeout(filterTimeout)
  filterTimeout = setTimeout(() => loadInitial(), 300)
}

function goToReview(reviewId: number) {
  router.push({ name: 'review', params: { id: reviewId } })
}

// Accepted Risks
async function loadRisks() {
  risksLoading.value = true
  risksError.value = ''
  risks.value = []
  risksHasMore.value = true
  expandedIssueId.value = null
  try {
    const [items, count] = await Promise.all([
      api.review.issuesByProject({ projectId, filters: risksFilters }),
      api.review.countIssuesByProject({ projectId, filters: risksFilters }),
    ])
    risks.value = items
    risksCount.value = count
    risksHasMore.value = items.length >= 50
    risksLoaded.value = true
  } catch (e) {
    risksError.value = e instanceof Error ? e.message : 'Failed to load accepted risks'
  } finally {
    risksLoading.value = false
  }
}

async function loadMoreRisks() {
  if (risksLoadingMore.value || !risksHasMore.value) return
  const lastId = risks.value[risks.value.length - 1]?.issueId
  if (!lastId) return

  risksLoadingMore.value = true
  try {
    const items = await api.review.issuesByProject({ projectId, filters: risksFilters, fromIssueId: lastId })
    risks.value.push(...items)
    risksHasMore.value = items.length >= 50
  } catch (e) {
    risksError.value = e instanceof Error ? e.message : 'Failed to load more risks'
  } finally {
    risksLoadingMore.value = false
  }
}

async function setRiskFeedback(issue: Issue, value: boolean | null) {
  try {
    await api.review.feedback({ issueId: issue.issueId, isFalsePositive: value ?? undefined })
    if (value !== true) {
      // Issue is no longer a false positive — remove from list
      risks.value = risks.value.filter(i => i.issueId !== issue.issueId)
      if (risksCount.value !== null) risksCount.value--
    }
  } catch (e) {
    risksError.value = e instanceof Error ? e.message : 'Failed to update feedback'
  }
}

function switchToRisks() {
  activeTab.value = 'risks'
  if (!risksLoaded.value) loadRisks()
}

onMounted(async () => {
  await loadInitial()
  // Pre-load risks count for the tab badge
  api.review.countIssuesByProject({ projectId, filters: risksFilters })
    .then(count => { risksCount.value = count })
    .catch(() => {})
})
</script>
