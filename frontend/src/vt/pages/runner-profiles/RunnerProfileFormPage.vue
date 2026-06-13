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
        <VInput v-model="entity.model" type="text" :placeholder="modelPlaceholder" />
        <p class="mt-1 text-xs text-fg-subtle">Leave empty to use the runner default.</p>
      </FormField>

      <FormField label="Effort" :error="fieldError('effort')">
        <VSelect v-model="entity.effort">
          <option :value="''">— default —</option>
          <option v-for="e in efforts" :key="e" :value="e">{{ e }}</option>
        </VSelect>
      </FormField>

      <template v-if="entity.runner === 'direct'">
        <FormField label="API Provider" :error="fieldError('apiProvider')">
          <VSelect v-model="entity.apiProvider">
            <option :value="''">— select —</option>
            <option v-for="p in providers" :key="p" :value="p">{{ p }}</option>
          </VSelect>
        </FormField>

        <FormField label="API Base URL" :error="fieldError('apiBaseURL')">
          <VInput v-model="entity.apiBaseURL" type="text" placeholder="https://api.deepseek.com" />
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
        <VInput v-model="tokenInput" type="password" autocomplete="new-password" :placeholder="entity.hasToken ? 'Saved — leave blank to keep' : 'Optional API key (env vars take priority)'" />
        <p class="mt-1 text-xs text-fg-subtle">
          Optional fallback API key, used only when the matching env var is absent.
          <span v-if="entity.hasToken" class="font-mono">Saved: {{ entity.tokenMasked }}</span>
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
import VSelect from '../../components/VSelect.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'
import VButton from '../../components/VButton.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const isEdit = computed(() => !!props.id)
const showConfirm = ref(false)

const runners = ['claude', 'opencode', 'codex', 'direct']
const efforts = ['low', 'medium', 'high', 'xhigh', 'max']
const providers = ['anthropic', 'deepseek', 'openai-compat']

const { entity, loading, saving, error, fieldError, load, save, remove } = useForm<RunnerProfile>(vtApi.runnerprofile, 'runnerProfile', () => ({
  id: 0, title: '', runner: 'claude', model: '', effort: '', apiProvider: '', apiBaseURL: '',
  params: { allowDangerousPermissions: false }, isDefault: false, statusId: 1, tokenMasked: '', hasToken: false,
}))

// Token is write-only: bind a separate input so an untouched field stays "keep".
const tokenInput = ref('')

const modelPlaceholder = computed(() => {
  switch (entity.runner) {
    case 'claude': return 'opus'
    case 'codex': return 'gpt-5.1-codex'
    case 'direct': return 'claude-opus-4-8, deepseek-v4-pro...'
    default: return 'provider/model'
  }
})

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
