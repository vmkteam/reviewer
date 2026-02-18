<template>
  <div class="bg-white rounded-xl border border-gray-200 shadow-sm overflow-x-auto">
    <table class="min-w-full">
      <thead>
        <tr class="border-b border-gray-100">
          <th
            class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider whitespace-nowrap"
            :class="sortable ? 'cursor-pointer select-none hover:text-gray-600 transition-colors' : ''"
            @click="sortable && toggleSort('severity')"
          >Severity {{ sortable ? sortIcon('severity') : '' }}</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">Title</th>
          <th
            class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider hidden md:table-cell whitespace-nowrap"
            :class="sortable ? 'cursor-pointer select-none hover:text-gray-600 transition-colors' : ''"
            @click="sortable && toggleSort('file')"
          >File {{ sortable ? sortIcon('file') : '' }}</th>
          <th
            class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider whitespace-nowrap"
            :class="sortable ? 'cursor-pointer select-none hover:text-gray-600 transition-colors' : ''"
            @click="sortable && toggleSort('issueType')"
          >Type {{ sortable ? sortIcon('issueType') : '' }}</th>
          <th v-if="showReviewType" class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider">RT</th>
          <th class="px-4 py-3 text-left text-[11px] font-semibold text-gray-400 uppercase tracking-wider hidden sm:table-cell">Feedback</th>
        </tr>
      </thead>
      <tbody>
        <template v-for="issue in displayIssues" :key="issue.issueId">
          <tr
            :id="'issue-' + issue.issueId"
            class="border-b border-gray-50 hover:bg-blue-50/30 cursor-pointer transition-colors row-hover"
            :class="expandedId === issue.issueId ? 'bg-blue-50/40' : ''"
            @click="onToggle(issue.issueId)"
          >
            <td class="px-4 py-3">
              <SeverityBadge :severity="issue.severity" />
            </td>
            <td class="px-4 py-3 text-sm text-gray-800">
              <span class="block max-w-[150px] sm:max-w-xs" :class="titleClass" :title="issue.title">{{ issue.title }}</span>
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
            <td v-if="showReviewType" class="px-4 py-3">
              <InfoBadge>{{ reviewTypeLabel(issue.reviewType) }}</InfoBadge>
            </td>
            <td class="px-4 py-3 hidden sm:table-cell" @click.stop>
              <div class="flex items-center gap-1">
                <FeedbackButtons
                  :is-false-positive="issue.isFalsePositive"
                  @feedback="$emit('feedback', issue, $event)"
                />
                <button
                  v-if="showCopyLink"
                  class="px-1.5 py-1 text-xs rounded-md border border-gray-200 text-gray-300 hover:text-gray-500 hover:border-gray-300 transition-all fb-btn ml-1"
                  :title="copiedIssueId === issue.issueId ? 'Copied!' : 'Copy link'"
                  @click="$emit('copyLink', issue.issueId)"
                >
                  <svg v-if="copiedIssueId !== issue.issueId" class="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1"/></svg>
                  <svg v-else class="w-3.5 h-3.5 text-emerald-500" fill="none" stroke="currentColor" viewBox="0 0 24 24"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/></svg>
                </button>
              </div>
            </td>
          </tr>
          <!-- Expanded detail row -->
          <tr v-if="expandedId === issue.issueId" class="bg-gray-50/60">
            <td :colspan="colspan" class="px-0 py-0">
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
                <!-- Feedback (mobile) -->
                <div class="sm:hidden flex items-center gap-2 pt-2 border-t border-gray-100" @click.stop>
                  <span class="text-xs text-gray-400">Feedback</span>
                  <FeedbackButtons
                    :is-false-positive="issue.isFalsePositive"
                    @feedback="$emit('feedback', issue, $event)"
                  />
                </div>
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
        <tr v-if="displayIssues.length === 0">
          <td :colspan="colspan" class="px-4 py-12 text-center text-sm text-gray-400">{{ emptyText }}</td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed } from 'vue'
import api, { type Issue, type Project } from '../api/factory'
import { ApiRpcError } from '../api/errors'
import SeverityBadge from './SeverityBadge.vue'
import MarkdownContent from './MarkdownContent.vue'
import PTextarea from './PTextarea.vue'
import InfoBadge from './InfoBadge.vue'
import FeedbackButtons from './FeedbackButtons.vue'
import { useFormat } from '../composables/useFormat'

const { reviewTypeLabel, buildVcsFileURL, compareSeverity } = useFormat()

const props = withDefaults(defineProps<{
  issues: Issue[]
  project: Project | null
  sortable?: boolean
  showReviewType?: boolean
  showCopyLink?: boolean
  copiedIssueId?: number | null
  titleClass?: string
  emptyText?: string
  expandedId?: number | null
}>(), {
  sortable: false,
  showReviewType: false,
  showCopyLink: false,
  copiedIssueId: null,
  titleClass: 'line-clamp-1',
  emptyText: 'No issues found.',
  expandedId: null,
})

const emit = defineEmits<{
  feedback: [issue: Issue, value: boolean | null]
  copyLink: [issueId: number]
  'update:expandedId': [id: number | null]
}>()

// Sorting (active only when sortable)
const sortField = ref<'severity' | 'file' | 'issueType'>('severity')
const sortAsc = ref(true)

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

const displayIssues = computed(() => {
  if (!props.sortable) return props.issues
  const copy = [...props.issues]
  copy.sort((a, b) => {
    let cmp = 0
    if (sortField.value === 'severity') cmp = compareSeverity(a.severity, b.severity)
    else if (sortField.value === 'file') cmp = a.file.localeCompare(b.file)
    else if (sortField.value === 'issueType') cmp = a.issueType.localeCompare(b.issueType)
    return sortAsc.value ? cmp : -cmp
  })
  return copy
})

const colspan = computed(() => {
  let cols = 5 // severity, title, file, type, feedback
  if (props.showReviewType) cols++
  return cols
})

// Comments (encapsulated)
const commentTexts = reactive<Record<number, string>>({})
const commentOriginals = reactive<Record<number, string>>({})
const commentSaving = reactive<Record<number, boolean>>({})
const commentErrors = reactive<Record<number, string>>({})

function isCommentDirty(issueId: number): boolean {
  return (commentTexts[issueId] ?? '') !== (commentOriginals[issueId] ?? '')
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

function onToggle(id: number) {
  if (props.expandedId === id) {
    emit('update:expandedId', null)
  } else {
    emit('update:expandedId', id)
    const issue = props.issues.find(i => i.issueId === id)
    if (issue && !(id in commentTexts)) {
      commentTexts[id] = issue.comment ?? ''
      commentOriginals[id] = issue.comment ?? ''
    }
  }
}
</script>
