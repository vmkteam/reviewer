<template>
  <div>
    <!-- Back + Header -->
    <div class="mb-6">
      <router-link to="/" class="inline-flex items-center gap-1 text-sm text-gray-400 hover:text-blue-600 transition-colors mb-3">
        <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 19l-7-7 7-7"/></svg>
        Projects
      </router-link>
      <div v-if="project" class="flex items-center gap-3 flex-wrap">
        <h1 class="text-2xl font-bold text-gray-900">{{ project.title }}</h1>
        <span class="badge bg-gray-100 text-gray-600">{{ project.language }}</span>
        <a
          v-if="project.vcsURL"
          :href="project.vcsURL"
          target="_blank"
          class="ml-auto text-sm text-blue-600 hover:text-blue-800 hover:underline transition-colors"
        >Open VCS</a>
      </div>
    </div>

    <!-- Tabs -->
    <div class="flex border-b border-gray-200 mb-5">
      <button
        @click="activeTab = 'reviews'"
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors"
        :class="activeTab === 'reviews'
          ? 'border-blue-600 text-blue-600'
          : 'border-transparent text-gray-400 hover:text-gray-600'"
      >
        Reviews
        <span v-if="totalCount !== null" class="ml-1 text-xs" :class="activeTab === 'reviews' ? 'text-blue-400' : 'text-gray-300'">({{ totalCount }})</span>
      </button>
      <button
        @click="switchToRisks"
        class="px-4 py-2.5 text-sm font-medium border-b-2 transition-colors"
        :class="activeTab === 'risks'
          ? 'border-blue-600 text-blue-600'
          : 'border-transparent text-gray-400 hover:text-gray-600'"
      >
        Accepted Risks
        <span v-if="risksCount !== null" class="ml-1 text-xs" :class="activeTab === 'risks' ? 'text-blue-400' : 'text-gray-300'">({{ risksCount }})</span>
      </button>
    </div>

    <!-- Reviews tab -->
    <div v-show="activeTab === 'reviews'">
      <!-- Filters -->
      <div class="flex flex-wrap items-center gap-3 mb-5">
        <input
          v-model="filters.author"
          type="text"
          placeholder="Filter by author..."
          class="w-full sm:w-44 px-3 py-1.5 text-sm bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors placeholder:text-gray-300"
          @input="onFilterChange"
        />
        <select
          v-model="filters.trafficLight"
          class="px-3 py-1.5 text-sm bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors"
          @change="onFilterChange"
        >
          <option value="">All statuses</option>
          <option value="green">Green</option>
          <option value="yellow">Yellow</option>
          <option value="red">Red</option>
        </select>
      </div>

      <!-- Loading -->
      <div v-if="loadingInitial" class="flex justify-center py-16">
        <div class="spinner spinner-lg" />
      </div>

      <!-- Error -->
      <div v-else-if="error" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ error }}</div>

      <!-- Table -->
      <div v-else>
        <InfiniteScroll :loading="loadingMore" :has-more="hasMore" @load-more="loadMore">
          <div class="bg-white rounded-xl border border-gray-200 overflow-hidden shadow-sm">
            <table class="min-w-full">
              <thead>
                <tr class="border-b border-gray-100">
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider w-10"></th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Title</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Author</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider hidden lg:table-cell">Branch</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">A/C/S/T</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Issues</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Date</th>
                </tr>
              </thead>
              <tbody>
                <tr
                  v-for="r in reviews"
                  :key="r.reviewId"
                  class="border-b border-gray-50 last:border-b-0 hover:bg-blue-50/40 cursor-pointer transition-colors row-hover"
                  @click="goToReview(r.reviewId)"
                >
                  <td class="px-4 py-3.5">
                    <TrafficLight :color="r.trafficLight" />
                  </td>
                  <td class="px-4 py-3.5">
                    <div class="text-sm font-medium text-gray-900">{{ r.title }}</div>
                    <div class="text-xs text-gray-300 mt-0.5">{{ r.externalId }}</div>
                  </td>
                  <td class="px-4 py-3.5 text-sm text-gray-600">{{ r.author }}</td>
                  <td class="px-4 py-3.5 hidden lg:table-cell">
                    <div class="flex items-center gap-1 text-xs">
                      <span class="font-mono text-gray-500 truncate max-w-[180px]">{{ r.sourceBranch }}</span>
                      <svg class="w-3 h-3 text-gray-300 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
                      <span class="font-mono text-gray-500 truncate max-w-[180px]">{{ r.targetBranch }}</span>
                    </div>
                  </td>
                  <td class="px-4 py-3.5">
                    <ReviewTypeDots :review-files="r.reviewFiles" />
                  </td>
                  <td class="px-4 py-3.5">
                    <span class="text-sm tabular-nums" :class="totalIssues(r) > 0 ? 'text-gray-700 font-medium' : 'text-gray-300'">
                      {{ totalIssues(r) || '—' }}
                    </span>
                  </td>
                  <td class="px-4 py-3.5 text-xs text-gray-400">
                    <TimeAgo :date="r.createdAt" />
                  </td>
                </tr>
                <tr v-if="reviews.length === 0">
                  <td colspan="7" class="px-4 py-12 text-center text-sm text-gray-400">No reviews found.</td>
                </tr>
              </tbody>
            </table>
          </div>
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
      <div v-else-if="risksError" class="rounded-lg border border-red-200 bg-red-50 p-4 text-sm text-red-700">{{ risksError }}</div>

      <!-- Risks table -->
      <div v-else>
        <InfiniteScroll :loading="risksLoadingMore" :has-more="risksHasMore" @load-more="loadMoreRisks">
          <div class="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
            <table class="min-w-full">
              <thead>
                <tr class="border-b border-gray-100">
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Severity</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Title</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider hidden md:table-cell">File</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Type</th>
                  <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Feedback</th>
                </tr>
              </thead>
              <tbody>
                <template v-for="issue in risks" :key="issue.issueId">
                  <tr
                    class="border-b border-gray-50 hover:bg-blue-50/30 cursor-pointer transition-colors"
                    :class="expandedIssueId === issue.issueId ? 'bg-blue-50/40' : ''"
                    @click="toggleIssueDetail(issue.issueId)"
                  >
                    <td class="px-4 py-3">
                      <SeverityBadge :severity="issue.severity" />
                    </td>
                    <td class="px-4 py-3 text-sm text-gray-800 max-w-xs">
                      <span class="line-clamp-1">{{ issue.title }}</span>
                    </td>
                    <td class="px-4 py-3 hidden md:table-cell" @click.stop>
                      <div class="text-xs font-mono text-gray-500">
                        <a
                          v-if="project?.vcsURL && issue.commitHash"
                          :href="buildVcsFileURL(project.vcsURL, issue.commitHash, issue.file, issue.lines)"
                          target="_blank"
                          class="text-blue-600 hover:text-blue-800 hover:underline"
                        >{{ issue.file }}<span v-if="issue.lines" class="text-gray-400">:{{ issue.lines }}</span></a>
                        <template v-else>{{ issue.file }}<span v-if="issue.lines" class="text-gray-300">:{{ issue.lines }}</span></template>
                      </div>
                    </td>
                    <td class="px-4 py-3 text-xs text-gray-500">{{ issue.issueType }}</td>
                    <td class="px-4 py-3" @click.stop>
                      <div class="flex items-center gap-1">
                        <button
                          class="px-2 py-1 text-xs rounded-md border transition-all"
                          :class="issue.isFalsePositive === false
                            ? 'bg-emerald-50 border-emerald-300 text-emerald-700 font-medium'
                            : 'border-gray-200 text-gray-400 hover:border-emerald-300 hover:text-emerald-600 hover:bg-emerald-50/50'"
                          @click="setRiskFeedback(issue, false)"
                          title="Confirmed issue"
                        >Valid</button>
                        <button
                          class="px-2 py-1 text-xs rounded-md border transition-all"
                          :class="issue.isFalsePositive === true
                            ? 'bg-red-50 border-red-300 text-red-700 font-medium'
                            : 'border-gray-200 text-gray-400 hover:border-red-300 hover:text-red-600 hover:bg-red-50/50'"
                          @click="setRiskFeedback(issue, true)"
                          title="False positive"
                        >FP</button>
                        <button
                          class="px-1.5 py-1 text-xs rounded-md border border-gray-200 text-gray-300 hover:text-gray-500 hover:border-gray-300 transition-all"
                          @click="setRiskFeedback(issue, null)"
                          title="Reset"
                        >&times;</button>
                      </div>
                    </td>
                  </tr>
                  <!-- Expanded detail row -->
                  <tr v-if="expandedIssueId === issue.issueId" class="bg-gray-50/60">
                    <td colspan="5" class="px-0 py-0">
                      <div class="px-6 py-5 border-t border-gray-100 space-y-3">
                        <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500">
                          <span class="badge bg-gray-100 text-gray-600">{{ issue.issueType }}</span>
                          <span class="badge bg-gray-100 text-gray-600">{{ issue.reviewType }}</span>
                          <span class="font-mono">
                            <a
                              v-if="project?.vcsURL && issue.commitHash"
                              :href="buildVcsFileURL(project.vcsURL, issue.commitHash, issue.file, issue.lines)"
                              target="_blank"
                              class="text-blue-600 hover:text-blue-800 hover:underline"
                            >{{ issue.file }}<span v-if="issue.lines" class="text-gray-400">:{{ issue.lines }}</span></a>
                            <template v-else>{{ issue.file }}<span v-if="issue.lines">:{{ issue.lines }}</span></template>
                          </span>
                        </div>
                        <p v-if="issue.description" class="text-sm text-gray-600 leading-relaxed">{{ issue.description }}</p>
                        <MarkdownContent v-if="issue.content" :content="issue.content" />
                        <!-- Comment -->
                        <div class="flex flex-col sm:flex-row gap-2 pt-2 border-t border-gray-100" @click.stop>
                          <textarea
                            v-model="commentTexts[issue.issueId]"
                            placeholder="Add comment..."
                            maxlength="255"
                            rows="2"
                            class="flex-1 px-3 py-2 text-sm bg-white border border-gray-200 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500/20 focus:border-blue-400 transition-colors resize-none placeholder:text-gray-300"
                          />
                          <div class="flex items-start gap-2">
                            <button
                              class="px-3 py-1.5 text-sm font-medium rounded-lg transition-colors disabled:opacity-50"
                              :class="isCommentDirty(issue.issueId)
                                ? 'text-white bg-blue-600 hover:bg-blue-700'
                                : 'text-gray-400 bg-gray-100 cursor-default'"
                              :disabled="commentSaving[issue.issueId] || !isCommentDirty(issue.issueId)"
                              @click="saveComment(issue)"
                            >{{ commentSaving[issue.issueId] ? 'Saving...' : 'Save' }}</button>
                            <span v-if="commentErrors[issue.issueId]" class="text-xs text-red-600 py-2">{{ commentErrors[issue.issueId] }}</span>
                          </div>
                        </div>
                      </div>
                    </td>
                  </tr>
                </template>
                <tr v-if="risks.length === 0">
                  <td colspan="5" class="px-4 py-12 text-center text-sm text-gray-400">No accepted risks found.</td>
                </tr>
              </tbody>
            </table>
          </div>
        </InfiniteScroll>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import api, { type ReviewSummary, type Project, type ReviewFilters, type Issue } from '../api/factory'
import { RpcError } from '../api/client'
import TrafficLight from '../components/TrafficLight.vue'
import ReviewTypeDots from '../components/ReviewTypeDots.vue'
import TimeAgo from '../components/TimeAgo.vue'
import InfiniteScroll from '../components/InfiniteScroll.vue'
import SeverityBadge from '../components/SeverityBadge.vue'
import MarkdownContent from '../components/MarkdownContent.vue'
import { setProjectCrumb } from '../utils/breadcrumbs'
import { buildVcsFileURL } from '../utils/format'

const props = defineProps<{ id: string }>()
const router = useRouter()

const projectId = parseInt(props.id, 10)
const project = ref<Project | null>(null)
const reviews = ref<ReviewSummary[]>([])
const totalCount = ref<number | null>(null)
const loadingInitial = ref(true)
const loadingMore = ref(false)
const hasMore = ref(true)
const error = ref('')
const activeTab = ref<'reviews' | 'risks'>('reviews')

const filters = reactive<{ author: string; trafficLight: string }>({
  author: '',
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

// Comments
const commentTexts = reactive<Record<number, string>>({})
const commentOriginals = reactive<Record<number, string>>({})
const commentSaving = reactive<Record<number, boolean>>({})
const commentErrors = reactive<Record<number, string>>({})

function isCommentDirty(issueId: number): boolean {
  return (commentTexts[issueId] ?? '') !== (commentOriginals[issueId] ?? '')
}

const risksFilters = { isFalsePositive: true }

function buildFilters(): ReviewFilters | undefined {
  const f: ReviewFilters = {}
  if (filters.author) f.author = filters.author
  if (filters.trafficLight) f.trafficLight = filters.trafficLight
  if (!f.author && !f.trafficLight) return undefined
  return f
}

async function loadInitial() {
  loadingInitial.value = true
  error.value = ''
  reviews.value = []
  hasMore.value = true
  try {
    const [projects, items, count] = await Promise.all([
      api.review.projects(),
      api.review.get(projectId, buildFilters()),
      api.review.count(projectId, buildFilters()),
    ])
    project.value = projects.find(p => p.projectId === projectId) ?? null
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
    const items = await api.review.get(projectId, buildFilters(), lastId)
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

function totalIssues(r: ReviewSummary): number {
  return r.reviewFiles.reduce((sum, rf) => sum + rf.issueStats.total, 0)
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
      api.review.issuesByProject(projectId, risksFilters),
      api.review.countIssuesByProject(projectId, risksFilters),
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
    const items = await api.review.issuesByProject(projectId, risksFilters, lastId)
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
    await api.review.feedback(issue.issueId, value)
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

function toggleIssueDetail(id: number) {
  if (expandedIssueId.value === id) {
    expandedIssueId.value = null
  } else {
    expandedIssueId.value = id
    const issue = risks.value.find(i => i.issueId === id)
    if (issue && !(id in commentTexts)) {
      commentTexts[id] = issue.comment ?? ''
      commentOriginals[id] = issue.comment ?? ''
    }
  }
}

async function saveComment(issue: Issue) {
  const id = issue.issueId
  commentErrors[id] = ''
  commentSaving[id] = true
  try {
    const text = commentTexts[id]?.trim() || ''
    await api.review.setComment(id, text || null)
    issue.comment = text || undefined
    commentOriginals[id] = commentTexts[id] ?? ''
  } catch (e) {
    commentErrors[id] = e instanceof RpcError ? e.message : 'Failed to save comment'
  } finally {
    commentSaving[id] = false
  }
}

onMounted(async () => {
  await loadInitial()
  // Pre-load risks count for the tab badge
  api.review.countIssuesByProject(projectId, risksFilters)
    .then(count => { risksCount.value = count })
    .catch(() => {})
})
</script>
