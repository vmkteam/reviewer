<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-fg">{{ isEdit ? 'Edit Runner Profile' : 'New Runner Profile' }}</h1>
      <div class="flex gap-2">
        <button v-if="isEdit" @click="showConfirm = true" class="p-2 text-fg-subtle hover:text-danger transition-colors" title="Delete"><svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" /></svg></button>
        <VButton variant="secondary" to="/runner-profiles">Cancel</VButton>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-12"><div class="spinner"></div></div>

    <form v-else @submit.prevent="handleSave" class="bg-surface rounded-xl border border-edge p-4 sm:p-6 max-w-3xl mx-auto">
      <p v-if="error" class="text-sm text-danger mb-4">{{ error }}</p>

      <FormField label="Title" :error="fieldError('title')">
        <VInput v-model="entity.title" type="text" />
      </FormField>

      <FormField label="Runner" :error="fieldError('runner')">
        <VSelect v-model="entity.runner">
          <option v-for="r in runners" :key="r" :value="r">{{ r }}</option>
        </VSelect>
      </FormField>

      <FormField label="Model" :error="fieldError('model')">
        <ComboInput v-model="entity.model" :suggestions="modelSuggestions" :placeholder="modelPlaceholder" />
        <p class="mt-1 text-xs text-fg-subtle">Leave empty to use the runner default.</p>
      </FormField>

      <FormField label="Effort" :error="fieldError('effort')">
        <VSelect v-model="entity.effort">
          <option :value="''">— default —</option>
          <option v-for="e in effortOptions" :key="e" :value="e">{{ e }}</option>
        </VSelect>
        <p v-if="!effortUsed" class="mt-1 text-xs text-fg-subtle">Ignored by this runner/provider.</p>
      </FormField>

      <template v-if="entity.runner === 'direct'">
        <FormField label="API Provider" :error="fieldError('apiProvider')">
          <VSelect v-model="entity.apiProvider">
            <option :value="''">— select —</option>
            <option v-for="p in providers" :key="p" :value="p">{{ p }}</option>
          </VSelect>
        </FormField>

        <FormField label="API Base URL" :error="fieldError('apiBaseURL')">
          <VInput v-model="entity.apiBaseURL" type="text" :placeholder="apiBaseURLPlaceholder" />
        </FormField>
      </template>

      <template v-if="entity.runner === 'opencode'">
        <FormField label="Permissions">
          <label class="flex items-center gap-2 text-sm text-fg-secondary">
            <input type="checkbox" v-model="entity.params.allowDangerousPermissions" class="rounded border-edge" />
            Allow dangerous permissions (--dangerously-skip-permissions)
          </label>
        </FormField>
      </template>

      <FormField label="Token" :error="fieldError('token')">
        <SecretInput v-model="tokenInput" placeholder="Optional API key (env vars take priority)" :saved-masked="entity.hasToken ? entity.tokenMasked : ''" />
        <p class="mt-1 text-xs text-fg-subtle">
          Optional fallback API key, used only when the matching env var is absent.
        </p>
      </FormField>

      <FormField label="Default">
        <label class="flex items-center gap-2 text-sm text-fg-secondary">
          <input type="checkbox" v-model="entity.isDefault" class="rounded border-edge" />
          Use this profile for projects without their own
        </label>
      </FormField>

      <FormField label="Status" :error="fieldError('statusId')">
        <StatusRadio v-model="entity.statusId" name="statusId" />
      </FormField>

      <div class="flex justify-end mt-6">
        <VButton type="submit" :disabled="saving">{{ saving ? 'Saving...' : 'Save' }}</VButton>
      </div>
    </form>

    <ConfirmDialog
      :open="showConfirm"
      title="Delete Runner Profile"
      message="Are you sure you want to delete this runner profile?"
      @confirm="handleDelete"
      @cancel="showConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type RunnerProfile } from '../../../api/vt'
import { useForm } from '../../composables/useForm'
import FormField from '../../components/FormField.vue'
import StatusRadio from '../../components/StatusRadio.vue'
import VInput from '../../components/VInput.vue'
import SecretInput from '../../components/SecretInput.vue'
import VSelect from '../../components/VSelect.vue'
import ComboInput from '../../components/ComboInput.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'
import VButton from '../../components/VButton.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const isEdit = computed(() => !!props.id)
const showConfirm = ref(false)

const runners = ['claude', 'opencode', 'codex', 'direct']
const efforts = ['low', 'medium', 'high', 'xhigh', 'max']
const providers = ['anthropic', 'deepseek', 'openai', 'openai-compat']

// Runner-dependent model suggestions for the datalist (free text still allowed).
// Keep in sync with ResolveDefaults and the price tables in codex.go /
// provider_factory.go. As of 2026-07.
const MODEL_SUGGESTIONS: Record<string, string[]> = {
  claude: ['opus', 'sonnet', 'haiku', 'claude-opus-5', 'claude-sonnet-5', 'claude-fable-5', 'claude-opus-4-8', 'claude-sonnet-4-6', 'claude-haiku-4-5'],
  codex: ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna', 'gpt-5.5', 'gpt-5.4', 'gpt-5.4-mini', 'gpt-5.3-codex', 'gpt-5.2-codex', 'gpt-5.1-codex-max', 'gpt-5.1-codex'],
  opencode: ['anthropic/claude-opus-5', 'anthropic/claude-opus-4-8', 'openai/gpt-5.6-sol', 'openai/gpt-5.5', 'deepseek/deepseek-v4-pro', 'deepseek/deepseek-v4-flash'],
}
// For the direct runner, suggestions depend on the selected API provider.
// deepseek-chat/reasoner are intentionally omitted — they retire 2026-07-24.
const DIRECT_MODEL_SUGGESTIONS: Record<string, string[]> = {
  anthropic: ['claude-opus-5', 'claude-sonnet-5', 'claude-fable-5', 'claude-opus-4-8', 'claude-sonnet-4-6', 'claude-haiku-4-5'],
  deepseek: ['deepseek-v4-pro', 'deepseek-v4-flash'],
  openai: ['gpt-5.6-sol', 'gpt-5.6-terra', 'gpt-5.6-luna', 'gpt-5.5', 'gpt-5.4', 'gpt-5.4-mini'],
  'openai-compat': [],
}
// Default API base URL per direct provider, shown as the input placeholder.
const PROVIDER_BASE_URL: Record<string, string> = {
  anthropic: 'https://api.anthropic.com',
  deepseek: 'https://api.deepseek.com',
  openai: 'https://api.openai.com/v1',
  'openai-compat': '',
}

const { entity, loading, saving, error, fieldError, load, save, remove } = useForm<RunnerProfile>(vtApi.runnerprofile, 'runnerProfile', () => ({
  id: 0, title: '', runner: 'claude', model: '', effort: '', apiProvider: '', apiBaseURL: '',
  params: { allowDangerousPermissions: false }, isDefault: false, statusId: 1, tokenMasked: '', hasToken: false,
}))

// Token is write-only: bind a separate input so an untouched field stays "keep".
const tokenInput = ref('')

// Model suggestions for the current runner (and provider, for direct). Free text
// stays allowed; this only populates the datalist and the placeholder.
const modelSuggestions = computed<string[]>(() =>
  entity.runner === 'direct'
    ? DIRECT_MODEL_SUGGESTIONS[entity.apiProvider ?? ''] ?? []
    : MODEL_SUGGESTIONS[entity.runner] ?? [],
)

const modelPlaceholder = computed(() => {
  if (modelSuggestions.value.length) return modelSuggestions.value[0]
  return entity.runner === 'opencode' ? 'provider/model' : 'model'
})

const apiBaseURLPlaceholder = computed(() => PROVIDER_BASE_URL[entity.apiProvider ?? ''] || 'https://...')

// Effort is honoured by the claude/codex CLIs and, for the direct runner, only by
// the Anthropic provider; deepseek/openai-compat and opencode ignore it. codex has
// no "max" level.
const effortUsed = computed(() => {
  if (entity.runner === 'claude' || entity.runner === 'codex') return true
  if (entity.runner === 'direct') return entity.apiProvider === 'anthropic'
  return false
})
const effortOptions = computed(() =>
  entity.runner === 'codex' ? efforts.filter((e) => e !== 'max') : efforts,
)

onMounted(() => {
  if (props.id) load(parseInt(props.id))
})

async function handleSave() {
  // Empty token input means "keep existing" (update) or "none" (add): omit it.
  entity.token = tokenInput.value ? tokenInput.value : undefined
  if (!entity.params) entity.params = { allowDangerousPermissions: false }
  if (await save()) router.push('/runner-profiles')
}

async function handleDelete() {
  showConfirm.value = false
  if (props.id && await remove(parseInt(props.id))) router.push('/runner-profiles')
}
</script>
