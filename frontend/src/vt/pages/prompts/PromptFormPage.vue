<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-fg">{{ isEdit ? 'Edit Prompt' : 'New Prompt' }}</h1>
      <div class="flex gap-2">
        <FillExampleSelect v-if="!isEdit" :presets="languagePresets" @select="fillExample" />
        <button v-if="isEdit" @click="showConfirm = true" class="p-2 text-fg-subtle hover:text-danger transition-colors" title="Delete"><svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" /></svg></button>
        <VButton variant="secondary" to="/prompts">Cancel</VButton>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-12"><div class="spinner"></div></div>

    <form v-else @submit.prevent="handleSave" class="bg-surface rounded-xl border border-edge p-4 sm:p-6 max-w-3xl mx-auto">
      <p v-if="error" class="text-sm text-danger mb-4">{{ error }}</p>

      <FormField label="Title" :error="fieldError('title')">
        <VInput v-model="entity.title" type="text" />
      </FormField>

      <FormField label="Common" :error="fieldError('common')">
        <VTextarea v-model="entity.common" :rows="4" />
      </FormField>

      <FormField label="Architecture" :error="fieldError('architecture')">
        <VTextarea v-model="entity.architecture" :rows="4" />
      </FormField>

      <FormField label="Code" :error="fieldError('code')">
        <VTextarea v-model="entity.code" :rows="4" />
      </FormField>

      <FormField label="Security" :error="fieldError('security')">
        <VTextarea v-model="entity.security" :rows="4" />
      </FormField>

      <FormField label="Tests" :error="fieldError('tests')">
        <VTextarea v-model="entity.tests" :rows="4" />
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
import VInput from '../../components/VInput.vue'
import VTextarea from '../../components/VTextarea.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'
import VButton from '../../components/VButton.vue'
import FillExampleSelect from '../../components/FillExampleSelect.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const isEdit = computed(() => !!props.id)
const showConfirm = ref(false)

const { entity, loading, saving, error, fieldError, load, save, remove } = useForm<Prompt>(vtApi.prompt, 'prompt', () => ({
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

const languagePresets: Record<string, { title: string; code: string; architecture: string; security: string; tests: string }> = {
  'Go': { title: 'Go Review', code: 'Rob Pike', architecture: 'Dave Cheney', security: 'Filippo Valsorda', tests: 'Mitchell Hashimoto' },
  'Swift / iOS': { title: 'Swift Review', code: 'Chris Lattner', architecture: 'Krzysztof Zabłocki', security: 'Philippe De Ryck', tests: 'John Sundell' },
  'Kotlin / Android': { title: 'Kotlin Review', code: 'Roman Elizarov', architecture: 'Yigit Boyar', security: 'Maddie Stone', tests: 'Jake Wharton' },
  'Vue + Nuxt': { title: 'Vue Review', code: 'Evan You', architecture: 'Daniel Roe', security: 'Liran Tal', tests: 'Jessica Sachs' },
  'Python': { title: 'Python Review', code: 'Raymond Hettinger', architecture: 'Sebastián Ramírez', security: 'Anthony Shaw', tests: 'Brian Okken' },
}

function fillExample(lang: string) {
  const preset = languagePresets[lang]
  if (!preset) return

  entity.title = preset.title
  entity.common = 'Дополнительно проверь текст задачи на предмет фикса.\nЕсли были исправлены ошибки, предположи, что к ним привело и где еще могут быть потенциальные ошибки.'
  entity.architecture = `${preset.architecture}. Есть ли ошибки в бизнес логике?`
  entity.code = `${preset.code}. Обязательно расскажи, что можно улучшить и упросить в данном коде.`
  entity.security = `${preset.security}. Проведи ревью безопасности этого MR.`
  entity.tests = `${preset.tests}. Проведи ревью тестов этого MR. Если тестов нет — укажи, какие нужно добавить и почему.`
  entity.statusId = 1
}
</script>
