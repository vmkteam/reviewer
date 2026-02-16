<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900">{{ isEdit ? 'Edit Prompt' : 'New Prompt' }}</h1>
      <div class="flex gap-2">
        <button v-if="!isEdit" type="button" @click="fillExample" class="px-4 py-2 text-sm font-medium text-amber-700 border border-amber-300 rounded-lg hover:bg-amber-50 transition-colors">Fill Example</button>
        <button v-if="isEdit" @click="showConfirm = true" class="p-2 text-gray-400 hover:text-red-600 transition-colors" title="Delete"><svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" /></svg></button>
        <router-link to="/prompts" class="px-4 py-2 text-sm font-medium text-gray-600 border border-gray-300 rounded-lg hover:bg-gray-50">Cancel</router-link>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-12"><div class="spinner"></div></div>

    <form v-else @submit.prevent="handleSave" class="bg-white rounded-xl border border-gray-200 p-4 sm:p-6 max-w-3xl mx-auto">
      <p v-if="error" class="text-sm text-red-600 mb-4">{{ error }}</p>

      <FormField label="Title" :error="fieldError('title')">
        <input v-model="entity.title" type="text" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500" />
      </FormField>

      <FormField label="Common" :error="fieldError('common')">
        <textarea v-model="entity.common" rows="4" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"></textarea>
      </FormField>

      <FormField label="Architecture" :error="fieldError('architecture')">
        <textarea v-model="entity.architecture" rows="4" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"></textarea>
      </FormField>

      <FormField label="Code" :error="fieldError('code')">
        <textarea v-model="entity.code" rows="4" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"></textarea>
      </FormField>

      <FormField label="Security" :error="fieldError('security')">
        <textarea v-model="entity.security" rows="4" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"></textarea>
      </FormField>

      <FormField label="Tests" :error="fieldError('tests')">
        <textarea v-model="entity.tests" rows="4" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"></textarea>
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
      title="Delete Prompt"
      message="Are you sure you want to delete this prompt?"
      @confirm="handleDelete"
      @cancel="showConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type Prompt } from '../../../api/vt'
import { useForm } from '../../composables/useForm'
import FormField from '../../components/FormField.vue'
import StatusRadio from '../../components/StatusRadio.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const isEdit = computed(() => !!props.id)
const showConfirm = ref(false)

const { entity, loading, saving, error, fieldError, load, save, remove } = useForm<Prompt>(vtApi.prompt, () => ({
  id: 0, title: '', common: '', architecture: '', code: '', security: '', tests: '', statusId: 1,
}))

onMounted(() => {
  if (props.id) load(parseInt(props.id))
})

async function handleSave() {
  if (await save()) router.push('/prompts')
}

async function handleDelete() {
  showConfirm.value = false
  if (props.id && await remove(parseInt(props.id))) router.push('/prompts')
}

function fillExample() {
  entity.title = 'Go Review'
  entity.common = 'Дополнительно проверь текст задачи на предмет фикса.\nЕсли были исправлены ошибки, предположи, что к ним привело и где еще могут быть потенциальные ошибки.'
  entity.architecture = 'Dave Cheney. Есть ли ошибки в бизнес логике?'
  entity.code = 'Rob Pike. Обязательно расскажи, что можно улучшить и упросить в данном коде.'
  entity.security = 'Filippo Valsorda. Проведи ревью безопасности этого MR.'
  entity.tests = 'Mitchell Hashimoto. Проведи ревью тестов этого MR. Если тестов нет — укажи, какие нужно добавить и почему.'
  entity.statusId = 1
}
</script>
