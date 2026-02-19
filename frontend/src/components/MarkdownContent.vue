<template>
  <div class="prose prose-sm max-w-none markdown-body" v-html="rendered" />
</template>

<script setup lang="ts">
import { computed } from 'vue'
import MarkdownIt from 'markdown-it'
import type StateCore from 'markdown-it/lib/rules_core/state_core.mjs'
import type Token from 'markdown-it/lib/token.mjs'
import hljs from 'highlight.js'
import 'highlight.js/styles/github.css'
import { buildTaskURL, getTaskPattern } from '../composables/useTaskLink'

const props = defineProps<{
  content: string
  taskTrackerUrl?: string | null
}>()

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

const rendered = computed(() => md.render(props.content || '', { taskTrackerURL: props.taskTrackerUrl }))
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
