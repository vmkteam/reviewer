<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900">{{ isEdit ? 'Edit Project' : 'New Project' }}</h1>
      <div class="flex gap-2">
        <button v-if="isEdit" @click="showConfirm = true" class="p-2 text-gray-400 hover:text-red-600 transition-colors" title="Delete"><svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" /></svg></button>
        <router-link to="/projects" class="px-4 py-2 text-sm font-medium text-gray-600 border border-gray-300 rounded-lg hover:bg-gray-50">Cancel</router-link>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-12"><div class="spinner"></div></div>

    <form v-else @submit.prevent="handleSave" class="bg-white rounded-xl border border-gray-200 p-6 max-w-3xl mx-auto">
      <p v-if="error" class="text-sm text-red-600 mb-4">{{ error }}</p>

      <FormField label="Title" :error="fieldError('title')">
        <VInput v-model="entity.title" type="text" />
      </FormField>

      <FormField label="VCS URL" :error="fieldError('vcsURL')">
        <VInput v-model="entity.vcsURL" @change="onVcsURLChange" type="text" placeholder="https://github.com/..." />
      </FormField>

      <FormField label="Language" :error="fieldError('language')">
        <VInput v-model="entity.language" type="text" placeholder="Go, TypeScript, etc." />
      </FormField>

      <FormField v-if="isEdit" label="Project Key">
        <VInput :model-value="entity.projectKey" type="text" readonly class="border-gray-200 bg-gray-50 text-gray-500" />
      </FormField>

      <FormField label="Prompt" :error="fieldError('promptId')">
        <FKSelect v-model="entity.promptId" :load-fn="loadPrompts" />
      </FormField>

      <FormField label="Task Tracker" :error="fieldError('taskTrackerId')">
        <FKSelect v-model="entity.taskTrackerId" :load-fn="loadTaskTrackers" nullable />
      </FormField>

      <FormField label="Slack Channel" :error="fieldError('slackChannelId')">
        <FKSelect v-model="entity.slackChannelId" :load-fn="loadSlackChannels" nullable />
      </FormField>

      <FormField label="Status" :error="fieldError('statusId')">
        <StatusRadio v-model="entity.statusId" name="statusId" />
      </FormField>

      <div class="flex justify-end mt-6">
        <button
          type="submit"
          :disabled="saving"
          class="px-6 py-2.5 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >{{ saving ? 'Saving...' : 'Save' }}</button>
      </div>
    </form>

    <ConfirmDialog
      :open="showConfirm"
      title="Delete Project"
      message="Are you sure you want to delete this project?"
      @confirm="handleDelete"
      @cancel="showConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type Project } from '../../../api/vt'
import { useForm } from '../../composables/useForm'
import FormField from '../../components/FormField.vue'
import StatusRadio from '../../components/StatusRadio.vue'
import FKSelect from '../../components/FKSelect.vue'
import VInput from '../../components/VInput.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const isEdit = computed(() => !!props.id)
const showConfirm = ref(false)

const { entity, loading, saving, error, fieldError, load, save, remove } = useForm<Project>(vtApi.project, 'project', () => ({
  id: 0, title: '', vcsURL: '', language: '', promptId: undefined, taskTrackerId: undefined, slackChannelId: undefined, statusId: 1,
}))

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

onMounted(() => {
  if (props.id) load(parseInt(props.id))
})

function onVcsURLChange() {
  if (entity.title) return
  try {
    const path = new URL(entity.vcsURL).pathname.replace(/^\/|\.git$/g, '')
    const parts = path.split('/')
    if (parts.length >= 2) {
      entity.title = parts.slice(-2).join('/')
    }
  } catch { /* ignore invalid URLs */ }
}

async function handleSave() {
  if (await save()) router.push('/projects')
}

async function handleDelete() {
  showConfirm.value = false
  if (props.id && await remove(parseInt(props.id))) router.push('/projects')
}
</script>
