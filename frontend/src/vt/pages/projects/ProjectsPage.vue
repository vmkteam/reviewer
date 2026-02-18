<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-fg">Projects</h1>
      <div class="flex items-center gap-2">
        <VButton variant="secondary" @click="openCI">CI</VButton>
        <VButton variant="secondary" to="/projects/bulk-add">Bulk Add</VButton>
        <VButton size="sm" to="/projects/new">Add Project</VButton>
      </div>
    </div>

    <SearchBar>
      <div>
        <label class="block text-xs font-medium text-fg-muted mb-1">Title</label>
        <VInput v-model="search.title" @input="applySearch" type="text" placeholder="Search..." />
      </div>
      <div>
        <label class="block text-xs font-medium text-fg-muted mb-1">Language</label>
        <VInput v-model="search.language" @input="applySearch" type="text" placeholder="Go, JS..." />
      </div>
      <div>
        <label class="block text-xs font-medium text-fg-muted mb-1">Status</label>
        <VSelect v-model="search.statusId" @change="applySearch">
          <option :value="undefined">All</option>
          <option :value="1">Enabled</option>
          <option :value="2">Disabled</option>
        </VSelect>
      </div>
    </SearchBar>

    <DataTable
      :columns="columns"
      :items="items"
      :loading="loading"
      :sort-column="viewOps.sortColumn"
      :sort-desc="viewOps.sortDesc"
      @sort="setSort"
      @row-click="(item: any) => router.push(`/projects/${item.id}`)"
    >
      <template #cell-title="{ item }">
        <span class="font-medium text-fg">{{ (item as ProjectSummary).title }}</span>
      </template>
      <template #cell-projectKey="{ item }">
        <button
          @click.stop="copyKey((item as ProjectSummary).projectKey)"
          class="font-mono text-xs px-1.5 py-0.5 rounded transition-colors cursor-pointer"
          :class="keyCopied === (item as ProjectSummary).projectKey ? 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300' : 'bg-edge-light text-fg-secondary hover:bg-accent-light hover:text-accent'"
          title="Copy to clipboard"
        >{{ keyCopied === (item as ProjectSummary).projectKey ? 'Copied!' : (item as ProjectSummary).projectKey }}</button>
      </template>
      <template #cell-prompt="{ item }">
        {{ (item as ProjectSummary).prompt?.title ?? '—' }}
      </template>
      <template #cell-taskTracker="{ item }">
        {{ (item as ProjectSummary).taskTracker?.title ?? '—' }}
      </template>
      <template #cell-slackChannel="{ item }">
        {{ (item as ProjectSummary).slackChannel?.title ?? '—' }}
      </template>
      <template #cell-status="{ item }">
        <StatusBadge :status-id="(item as ProjectSummary).status?.id" />
      </template>
      <template #cell-actions="{ item }">
        <button
          @click.stop="openLocalRun(item as ProjectSummary)"
          class="px-2.5 py-1 text-xs font-medium text-accent bg-accent-light border border-accent/20 rounded-md hover:bg-accent-light hover:border-accent/40 transition-colors"
        >Run</button>
      </template>
    </DataTable>

    <Pagination :page="viewOps.page" :page-size="viewOps.pageSize" :total="total" @update:page="setPage" />

    <!-- CI Setup Modal (general) -->
    <Teleport to="body">
      <div v-if="ciVisible" class="fixed inset-0 z-50 flex items-center justify-center" @keydown.esc="ciVisible = false" tabindex="-1" ref="ciDialogRef">
        <div class="fixed inset-0 bg-overlay" @click="ciVisible = false"></div>
        <div class="relative bg-surface rounded-xl shadow-xl max-w-2xl w-full mx-4 p-4 sm:p-6 max-h-[85vh] sm:max-h-[90vh] flex flex-col">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-fg">CI Setup</h3>
            <button @click="ciVisible = false" class="text-fg-subtle hover:text-fg-secondary text-xl leading-none">&times;</button>
          </div>

          <!-- Tabs -->
          <div class="flex border-b border-edge mb-4">
            <button
              @click="ciTab = 'review'"
              class="px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors"
              :class="ciTab === 'review' ? 'border-accent text-accent' : 'border-transparent text-fg-muted hover:text-fg-secondary'"
            >Review</button>
            <button
              @click="ciTab = 'dockerfile'"
              class="px-4 py-2 text-sm font-medium border-b-2 -mb-px transition-colors"
              :class="ciTab === 'dockerfile' ? 'border-accent text-accent' : 'border-transparent text-fg-muted hover:text-fg-secondary'"
            >Dockerfile</button>
          </div>

          <!-- Review tab -->
          <div v-if="ciTab === 'review'" class="flex flex-col gap-3 overflow-hidden">
            <div class="flex flex-col sm:flex-row items-start sm:items-center gap-2 sm:gap-4">
              <div class="text-xs text-fg-muted font-mono bg-surface-alt px-2 py-1 rounded truncate max-w-full">components/claude-code/templates/review.yml</div>
              <div class="flex items-center gap-2 shrink-0">
                <label class="text-xs text-fg-muted">Branch:</label>
                <input
                  v-model="ciTargetBranch"
                  @change="refreshCI"
                  type="text"
                  class="rounded border border-edge-strong px-2 py-1 text-xs w-full sm:w-24"
                />
              </div>
            </div>
            <div class="overflow-auto rounded-lg border border-edge bg-surface-alt flex-1">
              <pre class="p-3 text-xs leading-relaxed whitespace-pre overflow-x-auto"><code>{{ ciYaml }}</code></pre>
            </div>
            <VButton variant="secondary" size="sm" class="self-end" @click="copyToClipboard(ciYaml)">{{ ciCopied === 'review' ? 'Copied!' : 'Copy' }}</VButton>
          </div>

          <!-- Dockerfile tab -->
          <div v-if="ciTab === 'dockerfile'" class="flex flex-col gap-3 overflow-hidden">
            <div class="text-xs text-fg-muted font-mono bg-surface-alt px-2 py-1 rounded self-start">docker/claude-code/Dockerfile</div>
            <div class="overflow-auto rounded-lg border border-edge bg-surface-alt flex-1">
              <pre class="p-3 text-xs leading-relaxed whitespace-pre overflow-x-auto"><code>{{ dockerfile }}</code></pre>
            </div>
            <VButton variant="secondary" size="sm" class="self-end" @click="copyToClipboard(dockerfile, 'dockerfile')">{{ ciCopied === 'dockerfile' ? 'Copied!' : 'Copy' }}</VButton>
          </div>
        </div>
      </div>
    </Teleport>

    <!-- Local Run Modal (per-project) -->
    <Teleport to="body">
      <div v-if="localRunVisible" class="fixed inset-0 z-50 flex items-center justify-center" @keydown.esc="localRunVisible = false" tabindex="-1" ref="localRunDialogRef">
        <div class="fixed inset-0 bg-overlay" @click="localRunVisible = false"></div>
        <div class="relative bg-surface rounded-xl shadow-xl max-w-2xl w-full mx-4 p-4 sm:p-6 max-h-[85vh] sm:max-h-[90vh] flex flex-col">
          <div class="flex items-center justify-between mb-4">
            <h3 class="text-lg font-semibold text-fg">Local Run — {{ localRunProject?.title }}</h3>
            <button @click="localRunVisible = false" class="text-fg-subtle hover:text-fg-secondary text-xl leading-none">&times;</button>
          </div>

          <div class="flex flex-col gap-3 overflow-hidden">
            <div class="text-xs text-fg-muted font-mono bg-surface-alt px-2 py-1 rounded self-start">bash</div>
            <div class="overflow-auto rounded-lg border border-edge bg-surface-alt flex-1">
              <pre class="p-3 text-xs leading-relaxed whitespace-pre overflow-x-auto"><code>{{ localRunScript }}</code></pre>
            </div>
            <VButton variant="secondary" size="sm" class="self-end" @click="copyLocalRun">{{ localRunCopied ? 'Copied!' : 'Copy' }}</VButton>
          </div>
        </div>
      </div>
    </Teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, nextTick } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type ProjectSummary } from '../../../api/vt'
import { useCrud } from '../../composables/useCrud'
import DataTable from '../../components/DataTable.vue'
import Pagination from '../../components/Pagination.vue'
import SearchBar from '../../components/SearchBar.vue'
import VInput from '../../components/VInput.vue'
import VSelect from '../../components/VSelect.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import VButton from '../../components/VButton.vue'

const router = useRouter()
const { items, total, loading, viewOps, search, load, setSort, setPage, applySearch } = useCrud(vtApi.project, 'projectId')

const columns = [
  { key: 'id', label: 'ID', sortable: true, sortKey: 'projectId' },
  { key: 'title', label: 'Title', sortable: true },
  { key: 'language', label: 'Language', sortable: true },
  { key: 'projectKey', label: 'Key', sortable: true },
  { key: 'prompt', label: 'Prompt' },
  { key: 'taskTracker', label: 'Task Tracker' },
  { key: 'slackChannel', label: 'Slack Channel' },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'statusId' },
  { key: 'actions', label: '' },
]

// Copy project key
const keyCopied = ref('')
function copyKey(key: string) {
  navigator.clipboard.writeText(key)
  keyCopied.value = key
  setTimeout(() => { keyCopied.value = '' }, 2000)
}

// Modal refs
const ciDialogRef = ref<HTMLElement>()
const localRunDialogRef = ref<HTMLElement>()

// CI modal state (general)
const ciVisible = ref(false)
const ciTab = ref<'review' | 'dockerfile'>('review')
const ciYaml = ref('')
const ciTargetBranch = ref('devel')
const ciCopied = ref('')

// Local Run modal state (per-project)
const localRunVisible = ref(false)
const localRunProject = ref<ProjectSummary | null>(null)
const localRunCopied = ref(false)

const dockerfile = `FROM node:20-alpine
RUN apk add git bash curl
WORKDIR /app
RUN npm install -g @anthropic-ai/claude-code
RUN npm install -g marked

# Claude Code default settings
RUN mkdir -p /root/.claude && cat > /root/.claude/settings.json <<'EOF'
{
  "enabledPlugins": {
    "gopls-lsp@claude-plugins-official": true,
    "swift-lsp@claude-plugins-official": true
  },
  "attribution": {
    "commit": "",
    "pr": ""
  },
  "includeCoAuthoredBy": false,
  "permissions": {
    "deny": [
      "Read(**/.env)",
      "Bash(sudo:*)",
      "Bash(su:*)",
      "Bash(ssh:*)"
    ]
  },
  "language": "Russian",
  "autoUpdatesChannel": "latest",
  "gitAttribution": false,
  "env": {
    "CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC": 1,
    "DISABLE_TELEMETRY": 1,
    "DISABLE_ERROR_REPORTING": 1
  }
}
EOF

CMD ["claude-code"]`

const localRunScript = computed(() => {
  const baseURL = window.location.origin
  const key = localRunProject.value?.projectKey ?? 'YOUR_PROJECT_KEY'
  return `export PROJECT_KEY="${key}"
export REVIEWSRV_URL="${baseURL}"

# Download prompt
curl -sf "$REVIEWSRV_URL/v1/prompt/$PROJECT_KEY/" -o p.md

# Run claude-code review
claude \\
  --model opus \\
  --permission-mode acceptEdits \\
  --allowedTools "Bash(*) Read(*) Edit(*) Write(*) WebFetch(*)" \\
  -p "$(cat p.md)"

# Upload results
curl -sf "$REVIEWSRV_URL/v1/upload/upload.js" -o upload.cjs
REVIEW_DIR=. node upload.cjs

# Cleanup
rm -f p.md upload.cjs`
})

async function openCI() {
  ciTab.value = 'review'
  ciCopied.value = ''
  ciYaml.value = await vtApi.project.gitlabCI({ targetBranch: ciTargetBranch.value })
  ciVisible.value = true
  nextTick(() => ciDialogRef.value?.focus())
}

function openLocalRun(project: ProjectSummary) {
  localRunProject.value = project
  localRunCopied.value = false
  localRunVisible.value = true
  nextTick(() => localRunDialogRef.value?.focus())
}

async function refreshCI() {
  ciYaml.value = await vtApi.project.gitlabCI({ targetBranch: ciTargetBranch.value })
}

function copyToClipboard(text: string, tab: string = 'review') {
  navigator.clipboard.writeText(text)
  ciCopied.value = tab
  setTimeout(() => { ciCopied.value = '' }, 2000)
}

function copyLocalRun() {
  navigator.clipboard.writeText(localRunScript.value)
  localRunCopied.value = true
  setTimeout(() => { localRunCopied.value = false }, 2000)
}

onMounted(load)
</script>
