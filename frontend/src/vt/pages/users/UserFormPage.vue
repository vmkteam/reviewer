<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-fg">{{ isEdit ? 'Edit User' : 'New User' }}</h1>
      <div class="flex gap-2">
        <button v-if="isEdit" @click="showConfirm = true" class="p-2 text-fg-subtle hover:text-danger transition-colors" title="Delete"><svg xmlns="http://www.w3.org/2000/svg" class="h-5 w-5" viewBox="0 0 20 20" fill="currentColor"><path fill-rule="evenodd" d="M9 2a1 1 0 00-.894.553L7.382 4H4a1 1 0 000 2v10a2 2 0 002 2h8a2 2 0 002-2V6a1 1 0 100-2h-3.382l-.724-1.447A1 1 0 0011 2H9zM7 8a1 1 0 012 0v6a1 1 0 11-2 0V8zm5-1a1 1 0 00-1 1v6a1 1 0 102 0V8a1 1 0 00-1-1z" clip-rule="evenodd" /></svg></button>
        <VButton variant="secondary" to="/users">Cancel</VButton>
      </div>
    </div>

    <div v-if="loading" class="flex justify-center py-12"><div class="spinner"></div></div>

    <form v-else @submit.prevent="handleSave" class="bg-surface rounded-xl border border-edge p-6 max-w-3xl mx-auto">
      <p v-if="error" class="text-sm text-danger mb-4">{{ error }}</p>

      <FormField label="Login" :error="fieldError('login')">
        <VInput v-model="entity.login" type="text" />
      </FormField>

      <FormField label="Password" :error="fieldError('password')">
        <VInput v-model="entity.password" type="password" :placeholder="isEdit ? 'Leave empty to keep current' : 'Enter password'" />
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
      title="Delete User"
      message="Are you sure you want to delete this user?"
      @confirm="handleDelete"
      @cancel="showConfirm = false"
    />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type User } from '../../../api/vt'
import { useForm } from '../../composables/useForm'
import FormField from '../../components/FormField.vue'
import StatusRadio from '../../components/StatusRadio.vue'
import VInput from '../../components/VInput.vue'
import ConfirmDialog from '../../components/ConfirmDialog.vue'
import VButton from '../../components/VButton.vue'

const props = defineProps<{ id?: string }>()
const router = useRouter()
const isEdit = computed(() => !!props.id)
const showConfirm = ref(false)

const { entity, loading, saving, error, fieldError, load, save, remove } = useForm<User>(vtApi.user, 'user', () => ({
  id: 0, login: '', password: '', statusId: 1,
}))

onMounted(() => {
  if (props.id) load(parseInt(props.id))
})

async function handleSave() {
  if (await save()) router.push('/users')
}

async function handleDelete() {
  showConfirm.value = false
  if (props.id && await remove(parseInt(props.id))) router.push('/users')
}
</script>
