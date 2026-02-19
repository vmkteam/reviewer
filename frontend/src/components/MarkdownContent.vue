<template>
  <div class="prose prose-sm max-w-none markdown-body" v-html="rendered" @click="onClick" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import type StateCore from 'markdown-it/lib/rules_core/state_core.mjs'
import type Token from 'markdown-it/lib/token.mjs'
import hljs from 'highlight.js'
import 'highlight.js/styles/github.css'
import { buildTaskURL, getTaskPattern } from '../composables/useTaskLink'

export interface IssueBadgeInfo {
  issueId: number
  localId: string
  isFalsePositive?: boolean | null
  comment?: string
}

const props = defineProps<{
  content: string
  taskTrackerUrl?: string | null
  issues?: IssueBadgeInfo[]
}>()

const emit = defineEmits<{
  'goto-issue': [issueId: number]
}>()

const issueMap = computed(() => {
  const map = new Map<string, IssueBadgeInfo>()
  if (props.issues) {
    for (const iss of props.issues) {
      if (iss.localId) map.set(iss.localId, iss)
    }
  }
  return map
})

function onClick(e: MouseEvent) {
  const el = (e.target as HTMLElement).closest('[data-issue-id]')
  if (!el) return
  e.preventDefault()
  const id = parseInt((el as HTMLElement).dataset.issueId!, 10)
  if (id) emit('goto-issue', id)
}

function escapeHtml(str: string): string {
  return str.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;')
}

const md: MarkdownIt = new MarkdownIt({
  html: false,
  linkify: true,
  highlight(str: string, lang: string): string {
    if (lang && hljs.getLanguage(lang)) {
      try {
        return `<pre class="hljs"><code>${hljs.highlight(str, { language: lang }).value}</code></pre>`
      } catch { /* ignore */ }
    }
    return `<pre class="hljs"><code>${escapeHtml(str)}</code></pre>`
  },
})

md.core.ruler.push('task_link', (state: StateCore) => {
  const trackerURL = state.env?.taskTrackerURL as string | undefined
  if (!trackerURL) return

  const pattern = getTaskPattern(trackerURL)

  for (const blockToken of state.tokens) {
    if (blockToken.type !== 'inline' || !blockToken.children) continue

    const newChildren: Token[] = []
    let insideLink = false

    for (const token of blockToken.children) {
      if (token.type === 'link_open') { insideLink = true; newChildren.push(token); continue }
      if (token.type === 'link_close') { insideLink = false; newChildren.push(token); continue }
      if (token.type !== 'text' || insideLink || !pattern.test(token.content)) {
        newChildren.push(token)
        continue
      }

      // Split text by task pattern
      pattern.lastIndex = 0
      let lastIndex = 0
      let match: RegExpExecArray | null

      while ((match = pattern.exec(token.content)) !== null) {
        // Text before match
        if (match.index > lastIndex) {
          const textToken = new state.Token('text', '', 0)
          textToken.content = token.content.slice(lastIndex, match.index)
          newChildren.push(textToken)
        }

        // link_open
        const linkOpen = new state.Token('link_open', 'a', 1)
        linkOpen.attrSet('href', buildTaskURL(trackerURL, match[1]))
        linkOpen.attrSet('target', '_blank')
        linkOpen.attrSet('class', 'task-link')
        newChildren.push(linkOpen)

        // link text
        const linkText = new state.Token('text', '', 0)
        linkText.content = match[0]
        newChildren.push(linkText)

        // link_close
        const linkClose = new state.Token('link_close', 'a', -1)
        newChildren.push(linkClose)

        lastIndex = pattern.lastIndex
      }

      // Remaining text
      if (lastIndex < token.content.length) {
        const textToken = new state.Token('text', '', 0)
        textToken.content = token.content.slice(lastIndex)
        newChildren.push(textToken)
      }
    }

    blockToken.children = newChildren
  }
})

function buildBadgeHtml(issue: IssueBadgeInfo): string {
  let html = ' <span class="issue-badge">'
  if (issue.isFalsePositive === true) {
    html += '<span class="issue-badge-fp" title="False Positive">FP</span>'
  } else if (issue.isFalsePositive === false) {
    html += '<span class="issue-badge-valid" title="Confirmed">\u2713</span>'
  }
  html += `<a class="issue-badge-goto" data-issue-id="${issue.issueId}" title="Go to issue">\u2192\u00A0Issues</a>`
  html += '</span>'
  return html
}

function buildCommentHtml(comment: string): string {
  return `<div class="issue-comment-block">\uD83D\uDCAC ${escapeHtml(comment)}</div>\n`
}

md.core.ruler.push('issue_badge', (state: StateCore) => {
  const map = state.env?.issueMap as Map<string, IssueBadgeInfo> | undefined
  if (!map || map.size === 0) return

  const tokens = state.tokens
  // Iterate backwards so splice offsets don't shift
  for (let i = tokens.length - 2; i >= 0; i--) {
    if (tokens[i].type !== 'heading_open' || tokens[i].tag !== 'h3') continue
    const inlineToken = tokens[i + 1]
    if (!inlineToken || inlineToken.type !== 'inline' || !inlineToken.children) continue

    // Extract localId from the heading text content
    const textContent = inlineToken.children
      .filter(t => t.type === 'text')
      .map(t => t.content)
      .join('')
    const match = textContent.match(/^([ACST]\d+)\.\s/)
    if (!match) continue

    const issue = map.get(match[1])
    if (!issue) continue

    // Inline badge in the heading
    const badgeToken = new state.Token('html_inline', '', 0)
    badgeToken.content = buildBadgeHtml(issue)
    inlineToken.children.push(badgeToken)

    // Comment block after heading_close (tokens[i+2])
    if (issue.comment) {
      const commentToken = new state.Token('html_block', '', 0)
      commentToken.content = buildCommentHtml(issue.comment)
      tokens.splice(i + 3, 0, commentToken)
    }
  }
})

const rendered = computed(() => md.render(props.content || '', {
  taskTrackerURL: props.taskTrackerUrl,
  issueMap: issueMap.value,
}))
</script>

<style>
.markdown-body pre {
  background: var(--color-surface-alt);
  border-radius: 6px;
  padding: 16px;
  overflow-x: auto;
}
.markdown-body code {
  font-size: 0.85em;
}
.markdown-body pre code {
  padding: 0;
  background: transparent;
}
.markdown-body h1, .markdown-body h2, .markdown-body h3 {
  margin-top: 1.5em;
  margin-bottom: 0.5em;
}
.markdown-body ul, .markdown-body ol {
  padding-left: 1.5em;
}
.markdown-body table {
  border-collapse: collapse;
  width: 100%;
}
.markdown-body th, .markdown-body td {
  border: 1px solid var(--color-edge);
  padding: 6px 13px;
}
.markdown-body th {
  background: var(--color-surface-alt);
  font-weight: 600;
}
</style>
