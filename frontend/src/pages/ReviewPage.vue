<template>
  <div>
    <!-- Back link -->
    <router-link
      v-if="review"
      to="/"
      class="inline-flex items-center gap-1 text-sm text-fg-subtle hover:text-accent transition-colors mb-4"
    >
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M15 19l-7-7 7-7"/></svg>
      Back
    </router-link>

    <!-- Loading -->
    <div v-if="loading" class="flex justify-center py-16">
      <div class="spinner spinner-lg" />
    </div>

    <!-- Error -->
    <ErrorAlert v-else-if="error">{{ error }}</ErrorAlert>

    <template v-else-if="review">
      <!-- Header Card -->
      <div class="bg-surface rounded-xl border border-edge p-4 sm:p-6 mb-6 shadow-sm">
        <div class="flex items-start gap-4">
          <div class="mt-0.5">
            <TrafficLight :color="review.trafficLight" size="lg" />
          </div>
          <div class="flex-1 min-w-0">
            <h1 class="text-xl font-bold text-fg leading-snug">{{ review.title }}</h1>
            <p v-if="review.description" class="text-sm text-fg-muted mt-1 line-clamp-2">{{ review.description }}</p>
          </div>
          <ExternalLink
            v-if="review.externalId && review.externalId !== '0' && project?.vcsURL"
            :href="buildVcsMrURL(project.vcsURL, review.externalId)"
            class="flex-shrink-0"
          >{{ project.vcsURL.includes('github.com') ? 'PR' : 'MR' }} #{{ review.externalId }}</ExternalLink>
          <router-link
            v-if="review.lastVersionReviewId"
            :to="{ name: 'review', params: { id: review.lastVersionReviewId } }"
            class="flex-shrink-0 inline-flex items-center gap-1 px-3 py-1.5 text-sm font-medium text-amber-600 hover:text-amber-800 hover:bg-amber-50 dark:text-amber-400 dark:hover:text-amber-300 dark:hover:bg-amber-950 rounded-lg transition-colors"
          >
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6"/></svg>
            Latest version
          </router-link>
        </div>

        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-x-6 gap-y-4 mt-6 pt-5 border-t border-edge-light">
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Author</div>
            <div class="text-sm font-medium text-fg">{{ review.author }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Branch</div>
            <div class="flex items-center gap-1 text-xs font-mono text-fg-secondary min-w-0">
              <span class="truncate">{{ review.sourceBranch }}</span>
              <svg class="w-3 h-3 text-fg-faint flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
              <span class="truncate">{{ review.targetBranch }}</span>
            </div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Commit</div>
            <div class="text-sm font-mono text-fg-secondary">
              <a
                v-if="project?.vcsURL && review.commitHash"
                :href="buildVcsCommitURL(project.vcsURL, review.commitHash)"
                target="_blank"
                class="text-accent hover:text-accent-hover hover:underline"
              >{{ shortHash(review.commitHash) }}</a>
              <template v-else>{{ shortHash(review.commitHash) }}</template>
            </div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Date</div>
            <div class="text-sm text-fg-secondary">{{ formatDateTime(review.createdAt) }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Duration</div>
            <div class="text-sm text-fg-secondary">{{ formatDuration(review.durationMs) }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Model</div>
            <div class="text-sm text-fg-secondary">{{ review.modelInfo.model }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Tokens</div>
            <div class="text-sm text-fg-secondary tabular-nums">{{ review.modelInfo.inputTokens.toLocaleString() }} / {{ review.modelInfo.outputTokens.toLocaleString() }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-fg-subtle uppercase tracking-wider mb-1">Cost</div>
            <div class="text-sm text-fg-secondary">{{ formatCost(review.modelInfo.costUsd) }}</div>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <TabGroup :selected-index="selectedTab" @change="onTabChange">
        <TabList class="flex gap-1 border-b border-edge mb-6 overflow-x-auto">
          <Tab
            v-for="tab in tabs"
            :key="tab.key"
            v-slot="{ selected }"
            as="template"
          >
            <button
              class="relative px-4 py-2.5 text-sm font-medium rounded-t-lg focus:outline-none transition-colors whitespace-nowrap flex-shrink-0"
              :class="selected
                ? 'text-accent bg-accent-light/50'
                : 'text-fg-subtle hover:text-fg-secondary hover:bg-surface-alt'"
            >
              <span class="flex items-center gap-2">
                <TrafficLight v-if="tab.color" :color="tab.color" size="sm" />
                {{ tab.label }}
              </span>
              <span
                v-if="selected"
                class="absolute bottom-0 left-0 right-0 h-0.5 bg-accent rounded-full"
              />
            </button>
          </Tab>
        </TabList>

        <TabPanels>
          <!-- Review file tabs (A/C/S/T) -->
          <TabPanel v-for="rf in orderedReviewFiles" :key="rf.reviewFileId">
            <div class="bg-surface rounded-xl border border-edge shadow-sm overflow-hidden">
              <!-- Summary header -->
              <div class="px-4 sm:px-6 py-4 sm:py-5 border-b border-edge-light">
                <div class="flex items-start gap-3">
                  <div class="mt-0.5"><TrafficLight :color="rf.trafficLight" size="md" /></div>
                  <p class="text-sm text-fg-secondary leading-relaxed flex-1">{{ rf.summary }}</p>
                  <button
                    @click="downloadMarkdown(rf.content, rf.reviewType)"
                    class="shrink-0 p-1.5 text-fg-subtle hover:text-fg-secondary rounded-lg hover:bg-surface-alt transition-colors"
                    title="Download markdown"
                  >
                    <svg xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
                      <path stroke-linecap="round" stroke-linejoin="round" d="M3 16.5v2.25A2.25 2.25 0 005.25 21h13.5A2.25 2.25 0 0021 18.75V16.5M16.5 12L12 16.5m0 0L7.5 12m4.5 4.5V3" />
                    </svg>
                  </button>
                </div>
                <div class="mt-3 ml-6">
                  <IssueStatsBar :stats="rf.issueStats" />
                </div>
              </div>
              <!-- Content -->
              <div class="px-4 sm:px-6 py-4 sm:py-5">
                <MarkdownContent :content="rf.content" />
              </div>
            </div>
          </TabPanel>

          <!-- Issues tab -->
          <TabPanel>
            <!-- Filters -->
            <div class="flex flex-wrap items-center gap-3 mb-5">
              <PSelect v-model="issueFilters.severity">
                <option value="">All severities</option>
                <option value="critical">Critical</option>
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
              </PSelect>
              <PSelect v-model="issueFilters.issueType">
                <option value="">All issue types</option>
                <option v-for="it in issueTypes" :key="it" :value="it">{{ it }}</option>
              </PSelect>
              <PSelect v-model="issueFilters.reviewType">
                <option value="">All review types</option>
                <option value="architecture">Architecture</option>
                <option value="code">Code</option>
                <option value="security">Security</option>
                <option value="tests">Tests</option>
              </PSelect>
              <span v-if="issueCount !== null" class="ml-auto text-xs text-fg-subtle">
                {{ issueCount }} issue{{ issueCount !== 1 ? 's' : '' }}
              </span>
            </div>

            <!-- Loading issues -->
            <div v-if="issuesLoading" class="flex justify-center py-12">
              <div class="spinner" />
            </div>

            <!-- Issues table -->
            <IssuesTable
              v-else
              :issues="filteredIssues"
              :project="project"
              :sortable="true"
              :show-review-type="true"
              :show-copy-link="true"
              :copied-issue-id="copiedIssueId"
              title-class="truncate"
              :expanded-id="expandedIssueId"
              @feedback="setFeedback"
              @copy-link="copyIssueLink"
              @update:expanded-id="onExpandedIdChange"
            />
          </TabPanel>
        </TabPanels>
      </TabGroup>
    </template>

    <ScrollToTop />
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch, nextTick } from 'vue'
import { TabGroup, TabList, Tab, TabPanels, TabPanel } from '@headlessui/vue'
import api, { type Review, type Issue, type Project } from '../api/factory'
import TrafficLight from '../components/TrafficLight.vue'
import IssueStatsBar from '../components/IssueStatsBar.vue'
import MarkdownContent from '../components/MarkdownContent.vue'
import PSelect from '../components/PSelect.vue'
import ErrorAlert from '../components/ErrorAlert.vue'
import ExternalLink from '../components/ExternalLink.vue'
import ScrollToTop from '../components/ScrollToTop.vue'
import IssuesTable from '../components/IssuesTable.vue'
import { useFormat } from '../composables/useFormat'
import { useBreadcrumbs } from '../composables/useBreadcrumbs'

const { shortHash, formatDateTime, formatDuration, formatCost, reviewTypeFullName, buildVcsCommitURL, buildVcsMrURL } = useFormat()
const { setProject: setProjectCrumb, setReview: setReviewCrumb } = useBreadcrumbs()

const props = defineProps<{ id: string }>()

const reviewId = computed(() => parseInt(props.id, 10))
const review = ref<Review | null>(null)
const project = ref<Project | null>(null)
const loading = ref(true)
const error = ref('')

// Issues
const allIssues = ref<Issue[]>([])
const issuesLoading = ref(false)
const expandedIssueId = ref<number | null>(null)

const issueFilters = reactive<{ severity: string; issueType: string; reviewType: string }>({
  severity: '',
  issueType: '',
  reviewType: '',
})

// Tabs
const selectedTab = ref(0)
const typeOrder = ['architecture', 'code', 'security', 'tests']
const targetIssueId = ref<number | null>(null)
const copiedIssueId = ref<number | null>(null)

function parseHash(): { tabKey: string | null; issueId: number | null } {
  const hash = window.location.hash.replace('#', '')
  if (!hash) return { tabKey: null, issueId: null }
  const issueMatch = hash.match(/^issues-(\d+)$/)
  if (issueMatch) return { tabKey: 'issues', issueId: parseInt(issueMatch[1], 10) }
  return { tabKey: hash, issueId: null }
}

function applyHash() {
  const { tabKey, issueId } = parseHash()
  if (!tabKey) return
  const idx = tabs.value.findIndex(t => t.key === tabKey)
  if (idx >= 0) {
    selectedTab.value = idx
    if (tabKey === 'issues') {
      targetIssueId.value = issueId
    }
  }
}

function updateHash(hash: string) {
  history.replaceState(null, '', '#' + hash)
}

function copyIssueLink(issueId: number) {
  const url = window.location.origin + window.location.pathname + '#issues-' + issueId
  navigator.clipboard.writeText(url)
  copiedIssueId.value = issueId
  setTimeout(() => {
    if (copiedIssueId.value === issueId) copiedIssueId.value = null
  }, 1500)
}

function downloadMarkdown(content: string, reviewType: string) {
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' })
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = `${review.value?.title ?? 'review'}-${reviewType}.md`
  a.click()
  URL.revokeObjectURL(url)
}

async function scrollToIssue(issueId: number) {
  expandedIssueId.value = issueId
  // Wait for TabPanel switch + issue rows + expanded row to render
  await nextTick()
  await nextTick()
  const tryScroll = () => {
    const el = document.getElementById('issue-' + issueId)
    if (el) el.scrollIntoView({ behavior: 'smooth', block: 'center' })
  }
  tryScroll()
  // Fallback: HeadlessUI may need extra frames to render panel content
  setTimeout(tryScroll, 100)
}

const orderedReviewFiles = computed(() => {
  if (!review.value) return []
  return typeOrder
    .map(t => review.value!.reviewFiles.find(f => f.reviewType === t))
    .filter((f): f is NonNullable<typeof f> => !!f)
})

const tabs = computed(() => {
  const rfTabs = orderedReviewFiles.value.map(rf => ({
    key: rf.reviewType,
    label: reviewTypeFullName(rf.reviewType),
    color: rf.trafficLight,
  }))
  return [...rfTabs, { key: 'issues', label: 'Issues', color: '' }]
})

const issueTypes = computed(() => {
  const types = new Set(allIssues.value.map(i => i.issueType))
  return [...types].sort()
})

const filteredIssues = computed(() => {
  return allIssues.value.filter(i => {
    if (issueFilters.severity && i.severity !== issueFilters.severity) return false
    if (issueFilters.issueType && i.issueType !== issueFilters.issueType) return false
    if (issueFilters.reviewType && i.reviewType !== issueFilters.reviewType) return false
    return true
  })
})

const issueCount = computed(() => filteredIssues.value.length)

function onExpandedIdChange(id: number | null) {
  expandedIssueId.value = id
  if (id === null) {
    updateHash('issues')
  } else {
    updateHash('issues-' + id)
  }
}

async function loadIssues() {
  issuesLoading.value = true
  expandedIssueId.value = null
  try {
    allIssues.value = await api.review.issues({ reviewId: reviewId.value })
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load issues'
  } finally {
    issuesLoading.value = false
  }
}

async function setFeedback(issue: Issue, value: boolean | null) {
  try {
    await api.review.feedback({ issueId: issue.issueId, isFalsePositive: value ?? undefined })
    issue.isFalsePositive = value ?? undefined
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to update feedback'
  }
}

function onTabChange(index: number) {
  selectedTab.value = index
  const tab = tabs.value[index]
  if (tab) updateHash(tab.key)
  if (index === orderedReviewFiles.value.length) {
    loadIssues()
  }
}

async function loadProjectCrumb(projectId: number, rv: { reviewId: number; title: string }) {
  try {
    const projects = await api.review.projects()
    const p = projects.find(p => p.projectId === projectId)
    if (p) {
      project.value = p
      setProjectCrumb(p.projectId, p.title)
      // Re-set review crumb because setProjectCrumb clears it
      setReviewCrumb(rv.reviewId, rv.title)
    }
  } catch {
    // breadcrumb is non-critical, ignore errors
  }
}

onMounted(async () => {
  try {
    review.value = await api.review.getByID({ reviewId: reviewId.value })
    document.title = `${review.value.title} — reviewer`
    setReviewCrumb(review.value.reviewId, review.value.title)
    loadProjectCrumb(review.value.projectId, review.value)
    await loadIssues()
    // Apply hash after data is loaded — sets selectedTab and targetIssueId
    applyHash()
    // Wait for tab panel switch to render
    await nextTick()
    await nextTick()
    if (targetIssueId.value) {
      await scrollToIssue(targetIssueId.value)
      targetIssueId.value = null
    }
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load review'
  } finally {
    loading.value = false
  }
})

watch(() => props.id, async () => {
  loading.value = true
  error.value = ''
  try {
    review.value = await api.review.getByID({ reviewId: reviewId.value })
    document.title = `${review.value.title} — reviewer`
    setReviewCrumb(review.value.reviewId, review.value.title)
    loadProjectCrumb(review.value.projectId, review.value)
    selectedTab.value = 0
    await loadIssues()
  } catch (e) {
    error.value = e instanceof Error ? e.message : 'Failed to load review'
  } finally {
    loading.value = false
  }
})
</script>
