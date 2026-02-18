<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-fg">Bulk Add Projects</h1>
      <VButton variant="secondary" to="/projects">Cancel</VButton>
    </div>

    <form @submit.prevent="handleAdd" class="bg-surface rounded-xl border border-edge p-6 max-w-3xl mx-auto">
      <p v-if="error" class="text-sm text-danger mb-4">{{ error }}</p>

      <FormField label="VCS URLs (one per line)" :error="fieldErrors.vcsURLs">
        <textarea
          v-model="vcsURLs"
          rows="6"
          placeholder="https://github.com/org/repo1&#10;https://github.com/org/repo2"
          class="w-full rounded-lg border border-edge-strong px-3 py-2 text-sm focus:border-blue-500 focus:ring-1 focus:ring-accent outline-none font-mono"
          :disabled="adding"
        ></textarea>
      </FormField>

      <FormField label="Language" :error="fieldErrors.language">
        <VInput v-model="language" type="text" placeholder="Go, TypeScript, etc." :disabled="adding" />
      </FormField>

      <FormField label="Prompt" :error="fieldErrors.promptId">
        <FKSelect v-model="promptId" :load-fn="loadPrompts" />
      </FormField>

      <FormField label="Task Tracker">
        <FKSelect v-model="taskTrackerId" :load-fn="loadTaskTrackers" nullable />
      </FormField>

      <FormField label="Slack Channel">
        <FKSelect v-model="slackChannelId" :load-fn="loadSlackChannels" nullable />
      </FormField>

      <FormField label="Status">
        <StatusRadio v-model="statusId" name="bulkStatusId" />
      </FormField>

      <div class="flex justify-end mt-6">
        <VButton type="submit" :disabled="adding || !parsedURLs.length">{{ adding ? `${results.length ? 'Adding' : 'Validating'}... (${progress}/${parsedURLs.length})` : 'Add Projects' }}</VButton>
      </div>
    </form>

    <!-- Results table -->
    <div v-if="results.length" class="max-w-3xl mx-auto mt-6">
      <h2 class="text-lg font-semibold text-fg mb-3">Results</h2>
      <div class="bg-surface rounded-xl border border-edge overflow-x-auto">
        <table class="w-full text-sm">
          <thead>
            <tr class="bg-surface-alt border-b border-edge">
              <th class="text-left px-4 py-2 font-medium text-fg-secondary">VCS URL</th>
              <th class="text-left px-4 py-2 font-medium text-fg-secondary">Title</th>
              <th class="text-left px-4 py-2 font-medium text-fg-secondary">Status</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="r in results" :key="r.url" class="border-b border-edge-light last:border-0">
              <td class="px-4 py-2 font-mono text-xs text-fg-secondary break-all">{{ r.url }}</td>
              <td class="px-4 py-2 text-fg">{{ r.title }}</td>
              <td class="px-4 py-2">
                <span v-if="r.ok" class="text-green-700 dark:text-green-300 font-medium">OK</span>
                <span v-else class="text-danger">{{ r.error }}</span>
              </td>
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type Project } from '../../../api/vt'
import { ApiRpcError } from '../../../api/errors'
import { extractTitleFromVcsURL } from '../../composables/useVcsTitle'
import FormField from '../../components/FormField.vue'
import StatusRadio from '../../components/StatusRadio.vue'
import FKSelect from '../../components/FKSelect.vue'
import VInput from '../../components/VInput.vue'
import VButton from '../../components/VButton.vue'

interface BulkResult {
  url: string
  title: string
  ok: boolean
  error?: string
}

const router = useRouter()

const vcsURLs = ref('')
const language = ref('')
const promptId = ref<number | undefined>(undefined)
const taskTrackerId = ref<number | null | undefined>(undefined)
const slackChannelId = ref<number | null | undefined>(undefined)
const statusId = ref(1)

const adding = ref(false)
const progress = ref(0)
const error = ref('')
const fieldErrors = ref<{ vcsURLs?: string; language?: string; promptId?: string }>({})
const results = ref<BulkResult[]>([])

const parsedURLs = computed(() =>
  vcsURLs.value.split('\n').map(l => l.trim()).filter(l => l.length > 0)
)

async function loadPrompts() {
  const list = await vtApi.prompt.get({ viewOps: { page: 1, pageSize: 500, sortColumn: 'title', sortDesc: false } })
  return (list ?? []).map(p => ({ id: p.id, title: p.title }))
}

async function loadTaskTrackers() {
  const list = await vtApi.tasktracker.get({ viewOps: { page: 1, pageSize: 500, sortColumn: 'title', sortDesc: false } })
  return (list ?? []).map(t => ({ id: t.id, title: t.title }))
}

async function loadSlackChannels() {
  const list = await vtApi.slackchannel.get({ viewOps: { page: 1, pageSize: 500, sortColumn: 'title', sortDesc: false } })
  return (list ?? []).map(s => ({ id: s.id, title: s.title }))
}

async function handleAdd() {
  fieldErrors.value = {}
  error.value = ''

  const urls = parsedURLs.value
  let hasErrors = false
  if (!urls.length) {
    fieldErrors.value.vcsURLs = 'Enter at least one URL'
    hasErrors = true
  }
  if (!language.value.trim()) {
    fieldErrors.value.language = 'Required field'
    hasErrors = true
  }
  if (!promptId.value) {
    fieldErrors.value.promptId = 'Required field'
    hasErrors = true
  }
  if (hasErrors) return

  adding.value = true
  progress.value = 0
  results.value = []

  // Build project objects
  const projects = urls.map(url => ({
    url,
    title: extractTitleFromVcsURL(url),
    data: {
      id: 0,
      title: extractTitleFromVcsURL(url),
      vcsURL: url,
      language: language.value,
      promptId: promptId.value,
      taskTrackerId: taskTrackerId.value ?? undefined,
      slackChannelId: slackChannelId.value ?? undefined,
      statusId: statusId.value,
    } as unknown as Project,
  }))

  // Phase 1: validate all
  const allResults: BulkResult[] = []
  let hasValidationErrors = false

  for (const p of projects) {
    try {
      const validationErrors = await vtApi.project.validate({ project: p.data })
      if (validationErrors && validationErrors.length > 0) {
        const msgs = validationErrors.map(e => `${e.field}: ${e.error}`).join(', ')
        allResults.push({ url: p.url, title: p.title, ok: false, error: msgs })
        hasValidationErrors = true
      } else {
        allResults.push({ url: p.url, title: p.title, ok: true })
      }
    } catch (e: unknown) {
      allResults.push({ url: p.url, title: p.title, ok: false, error: errorMessage(e) })
      hasValidationErrors = true
    }
    progress.value++
  }

  // If any validation failed — show results, don't add anything
  if (hasValidationErrors) {
    results.value = allResults
    adding.value = false
    return
  }

  // Phase 2: add all
  progress.value = 0
  const addResults: BulkResult[] = []

  for (const p of projects) {
    try {
      await vtApi.project.add({ project: p.data })
      addResults.push({ url: p.url, title: p.title, ok: true })
    } catch (e: unknown) {
      addResults.push({ url: p.url, title: p.title, ok: false, error: errorMessage(e) })
    }
    progress.value++
  }

  results.value = addResults
  adding.value = false

  if (addResults.every(r => r.ok)) {
    router.push('/projects')
  }
}

function errorMessage(e: unknown): string {
  if (e instanceof ApiRpcError && e.data) {
    const data = e.data as Array<{ field: string; error: string }>
    if (Array.isArray(data)) {
      return data.map(fe => `${fe.field}: ${fe.error}`).join(', ')
    }
    return (e as Error).message
  }
  return e instanceof Error ? e.message : 'Unknown error'
}
</script>
