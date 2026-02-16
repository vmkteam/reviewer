<template>
  <div>
    <!-- Back link -->
    <router-link
      v-if="review"
      to="/"
      class="inline-flex items-center gap-1 text-sm text-gray-400 hover:text-blue-600 transition-colors mb-4"
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
      <div class="bg-white rounded-xl border border-gray-200 p-6 mb-6 shadow-sm">
        <div class="flex items-start gap-4">
          <div class="mt-0.5">
            <TrafficLight :color="review.trafficLight" size="lg" />
          </div>
          <div class="flex-1 min-w-0">
            <h1 class="text-xl font-bold text-gray-900 leading-snug">{{ review.title }}</h1>
            <p v-if="review.description" class="text-sm text-gray-500 mt-1 line-clamp-2">{{ review.description }}</p>
          </div>
          <a
            v-if="review.externalId && review.externalId !== '0' && project?.vcsURL"
            :href="buildVcsMrURL(project.vcsURL, review.externalId)"
            target="_blank"
            class="flex-shrink-0 inline-flex items-center gap-1 px-3 py-1.5 text-sm font-medium text-blue-600 hover:text-blue-800 hover:bg-blue-50 rounded-lg transition-colors"
          >
            {{ project.vcsURL.includes('github.com') ? 'PR' : 'MR' }} #{{ review.externalId }}
            <svg class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"/></svg>
          </a>
        </div>

        <div class="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 gap-x-6 gap-y-4 mt-6 pt-5 border-t border-gray-100">
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Author</div>
            <div class="text-sm font-medium text-gray-800">{{ review.author }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Branch</div>
            <div class="flex items-center gap-1 text-xs font-mono text-gray-600">
              <span class="truncate">{{ review.sourceBranch }}</span>
              <svg class="w-3 h-3 text-gray-300 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/></svg>
              <span class="truncate">{{ review.targetBranch }}</span>
            </div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Commit</div>
            <div class="text-sm font-mono text-gray-600">
              <a
                v-if="project?.vcsURL && review.commitHash"
                :href="buildVcsCommitURL(project.vcsURL, review.commitHash)"
                target="_blank"
                class="text-blue-600 hover:text-blue-800 hover:underline"
              >{{ shortHash(review.commitHash) }}</a>
              <template v-else>{{ shortHash(review.commitHash) }}</template>
            </div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Date</div>
            <div class="text-sm text-gray-600">{{ formatDateTime(review.createdAt) }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Duration</div>
            <div class="text-sm text-gray-600">{{ formatDuration(review.durationMs) }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Model</div>
            <div class="text-sm text-gray-600">{{ review.modelInfo.model }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Tokens</div>
            <div class="text-sm text-gray-600 tabular-nums">{{ review.modelInfo.inputTokens.toLocaleString() }} / {{ review.modelInfo.outputTokens.toLocaleString() }}</div>
          </div>
          <div>
            <div class="text-[11px] font-medium text-gray-400 uppercase tracking-wider mb-1">Cost</div>
            <div class="text-sm text-gray-600">{{ formatCost(review.modelInfo.costUsd) }}</div>
          </div>
        </div>
      </div>

      <!-- Tabs -->
      <TabGroup :selected-index="selectedTab" @change="onTabChange">
        <TabList class="flex gap-1 border-b border-gray-200 mb-6 overflow-x-auto">
          <Tab
            v-for="tab in tabs"
            :key="tab.key"
            v-slot="{ selected }"
            as="template"
          >
            <button
              class="relative px-4 py-2.5 text-sm font-medium rounded-t-lg focus:outline-none transition-colors whitespace-nowrap flex-shrink-0"
              :class="selected
                ? 'text-blue-600 bg-blue-50/50'
                : 'text-gray-400 hover:text-gray-600 hover:bg-gray-50'"
            >
              <span class="flex items-center gap-2">
                <TrafficLight v-if="tab.color" :color="tab.color" size="sm" />
                {{ tab.label }}
              </span>
              <span
                v-if="selected"
                class="absolute bottom-0 left-0 right-0 h-0.5 bg-blue-600 rounded-full"
              />
            </button>
          </Tab>
        </TabList>

        <TabPanels>
          <!-- Review file tabs (A/C/S/T) -->
          <TabPanel v-for="rf in orderedReviewFiles" :key="rf.reviewFileId">
            <div class="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
              <!-- Summary header -->
              <div class="px-6 py-5 border-b border-gray-100">
                <div class="flex items-start gap-3">
                  <div class="mt-0.5"><TrafficLight :color="rf.trafficLight" size="md" /></div>
                  <p class="text-sm text-gray-600 leading-relaxed">{{ rf.summary }}</p>
                </div>
                <div class="mt-3 ml-6">
                  <IssueStatsBar :stats="rf.issueStats" />
                </div>
              </div>
              <!-- Content -->
              <div class="px-6 py-5">
                <MarkdownContent :content="rf.content" />
              </div>
            </div>
          </TabPanel>

          <!-- Issues tab -->
          <TabPanel>
            <!-- Filters -->
            <div class="flex flex-wrap items-center gap-3 mb-5">
              <PSelect v-model="issueFilters.severity" @change="loadIssues">
                <option value="">All severities</option>
                <option value="critical">Critical</option>
                <option value="high">High</option>
                <option value="medium">Medium</option>
                <option value="low">Low</option>
              </PSelect>
              <PSelect v-model="issueFilters.issueType" @change="loadIssues">
                <option value="">All issue types</option>
                <option v-for="it in issueTypes" :key="it" :value="it">{{ it }}</option>
              </PSelect>
              <PSelect v-model="issueFilters.reviewType" @change="loadIssues">
                <option value="">All review types</option>
                <option value="architecture">Architecture</option>
                <option value="code">Code</option>
                <option value="security">Security</option>
                <option value="tests">Tests</option>
              </PSelect>
              <span v-if="issueCount !== null" class="ml-auto text-xs text-gray-400">
                {{ issueCount }} issue{{ issueCount !== 1 ? 's' : '' }}
              </span>
            </div>

            <!-- Loading issues -->
            <div v-if="issuesLoading" class="flex justify-center py-12">
              <div class="spinner" />
            </div>

            <!-- Issues table -->
            <div v-else class="bg-white rounded-xl border border-gray-200 shadow-sm overflow-hidden">
              <table class="min-w-full">
                <thead>
                  <tr class="border-b border-gray-100">
                    <th
                      class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider cursor-pointer select-none hover:text-gray-600 transition-colors"
                      @click="toggleSort('severity')"
                    >Severity {{ sortIcon('severity') }}</th>
                    <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Title</th>
                    <th
                      class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider cursor-pointer select-none hover:text-gray-600 transition-colors hidden md:table-cell"
                      @click="toggleSort('file')"
                    >File {{ sortIcon('file') }}</th>
                    <th
                      class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider cursor-pointer select-none hover:text-gray-600 transition-colors"
                      @click="toggleSort('issueType')"
                    >Type {{ sortIcon('issueType') }}</th>
                    <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">RT</th>
                    <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Feedback</th>
                  </tr>
                </thead>
                <tbody>
                  <template v-for="issue in sortedIssues" :key="issue.issueId">
                    <tr
                      :id="'issue-' + issue.issueId"
                      class="border-b border-gray-50 hover:bg-blue-50/30 cursor-pointer transition-colors row-hover"
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
                      <td class="px-4 py-3">
                        <InfoBadge>{{ reviewTypeLabel(issue.reviewType) }}</InfoBadge>
                      </td>
                      <td class="px-4 py-3" @click.stop>
                        <div class="flex items-center gap-1">
                          <FeedbackButtons
                            :is-false-positive="issue.isFalsePositive"
                            @feedback="setFeedback(issue, $event)"
                          />
                          <button
                            class="px-1.5 py-1 text-xs rounded-md border border-gray-200 text-gray-300 hover:text-gray-500 hover:border-gray-300 transition-all fb-btn ml-1"
                            :title="copiedIssueId === issue.issueId ? 'Copied!' : 'Copy link'"
                            @click="copyIssueLink(issue.issueId)"
                          >
                            <svg v-if="copiedIssueId !== issue.issueId" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/></svg>
                            <svg v-else class="w-3.5 h-3.5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
                          </button>
                        </div>
                      </td>
                    </tr>
                    <!-- Inline expanded detail row -->
                    <tr v-if="expandedIssueId === issue.issueId" class="bg-gray-50/60">
                      <td colspan="6" class="px-0 py-0">
                        <div class="px-6 py-5 border-t border-gray-100 space-y-3">
                          <div class="flex flex-wrap items-center gap-2 text-xs text-gray-500">
                            <InfoBadge>{{ issue.issueType }}</InfoBadge>
                            <InfoBadge>{{ reviewTypeLabel(issue.reviewType) }}</InfoBadge>
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
                            <PTextarea
                              v-model="commentTexts[issue.issueId]"
                              placeholder="Add comment..."
                              maxlength="255"
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
                  <tr v-if="sortedIssues.length === 0">
                    <td colspan="6" class="px-4 py-12 text-center text-sm text-gray-400">No issues found.</td>
                  </tr>
                </tbody>
              </table>
            </div>
          </TabPanel>
        </TabPanels>
      </TabGroup>
    </template>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch, nextTick } from 'vue'
import { TabGroup, TabList, Tab, TabPanels, TabPanel } from '@headlessui/vue'
import api, { type Review, type Issue, type IssueFilters, type Project } from '../api/factory'
import { ApiRpcError } from '../api/errors'
import TrafficLight from '../components/TrafficLight.vue'
import SeverityBadge from '../components/SeverityBadge.vue'
import IssueStatsBar from '../components/IssueStatsBar.vue'
import MarkdownContent from '../components/MarkdownContent.vue'
import PSelect from '../components/PSelect.vue'
import PTextarea from '../components/PTextarea.vue'
import InfoBadge from '../components/InfoBadge.vue'
import ErrorAlert from '../components/ErrorAlert.vue'
import FeedbackButtons from '../components/FeedbackButtons.vue'
import { shortHash, formatDateTime, formatDuration, formatCost, reviewTypeLabel, reviewTypeFullName, compareSeverity, buildVcsCommitURL, buildVcsFileURL, buildVcsMrURL } from '../utils/format'
import { setProjectCrumb, setReviewCrumb } from '../utils/breadcrumbs'

const props = defineProps<{ id: string }>()

const reviewId = computed(() => parseInt(props.id, 10))
const review = ref<Review | null>(null)
const project = ref<Project | null>(null)
const loading = ref(true)
const error = ref('')

// Issues
const issues = ref<Issue[]>([])
const issueCount = ref<number | null>(null)
const issuesLoading = ref(false)
const expandedIssueId = ref<number | null>(null)

const issueFilters = reactive<{ severity: string; issueType: string; reviewType: string }>({
  severity: '',
  issueType: '',
  reviewType: '',
})

const sortField = ref<'severity' | 'file' | 'issueType'>('severity')
const sortAsc = ref(true)

// Comments
const commentTexts = reactive<Record<number, string>>({})
const commentOriginals = reactive<Record<number, string>>({})
const commentSaving = reactive<Record<number, boolean>>({})
const commentErrors = reactive<Record<number, string>>({})

function isCommentDirty(issueId: number): boolean {
  return (commentTexts[issueId] ?? '') !== (commentOriginals[issueId] ?? '')
}


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

async function scrollToIssue(issueId: number) {
  expandedIssueId.value = issueId
  const issue = issues.value.find(i => i.issueId === issueId)
  if (issue && !(issueId in commentTexts)) {
    commentTexts[issueId] = issue.comment ?? ''
    commentOriginals[issueId] = issue.comment ?? ''
  }
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
  const types = new Set(issues.value.map(i => i.issueType))
  return [...types].sort()
})

const sortedIssues = computed(() => {
  const copy = [...issues.value]
  copy.sort((a, b) => {
    let cmp = 0
    if (sortField.value === 'severity') cmp = compareSeverity(a.severity, b.severity)
    else if (sortField.value === 'file') cmp = a.file.localeCompare(b.file)
    else if (sortField.value === 'issueType') cmp = a.issueType.localeCompare(b.issueType)
    return sortAsc.value ? cmp : -cmp
  })
  return copy
})

function toggleSort(field: 'severity' | 'file' | 'issueType') {
  if (sortField.value === field) {
    sortAsc.value = !sortAsc.value
  } else {
    sortField.value = field
    sortAsc.value = true
  }
}

function sortIcon(field: string): string {
  if (sortField.value !== field) return ''
  return sortAsc.value ? '\u25B2' : '\u25BC'
}

function toggleIssueDetail(id: number) {
  if (expandedIssueId.value === id) {
    expandedIssueId.value = null
    updateHash('issues')
  } else {
    expandedIssueId.value = id
    updateHash('issues-' + id)
    const issue = issues.value.find(i => i.issueId === id)
    if (issue && !(id in commentTexts)) {
      commentTexts[id] = issue.comment ?? ''
      commentOriginals[id] = issue.comment ?? ''
    }
  }
}

function buildIssueFilters(): IssueFilters | undefined {
  const f: IssueFilters = {}
  if (issueFilters.severity) f.severity = issueFilters.severity
  if (issueFilters.issueType) f.issueType = issueFilters.issueType
  if (issueFilters.reviewType) f.reviewType = issueFilters.reviewType
  if (!f.severity && !f.issueType && !f.reviewType) return undefined
  return f
}

async function loadIssues() {
  issuesLoading.value = true
  expandedIssueId.value = null
  try {
    const [items, count] = await Promise.all([
      api.review.issues({ reviewId: reviewId.value, filters: buildIssueFilters() }),
      api.review.countIssues({ reviewId: reviewId.value, filters: buildIssueFilters() }),
    ])
    issues.value = items
    issueCount.value = count
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

async function saveComment(issue: Issue) {
  const id = issue.issueId
  commentErrors[id] = ''
  commentSaving[id] = true
  try {
    const text = commentTexts[id]?.trim() || ''
    await api.review.setComment({ issueId: id, comment: text || undefined })
    issue.comment = text || undefined
    commentOriginals[id] = commentTexts[id] ?? ''
  } catch (e) {
    commentErrors[id] = e instanceof ApiRpcError ? e.message : 'Failed to save comment'
  } finally {
    commentSaving[id] = false
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
