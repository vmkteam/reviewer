<template>
  <div>
    <h1 class="text-xl sm:text-2xl font-bold text-gray-900 mb-6">Profile</h1>

    <div class="bg-white rounded-xl border border-gray-200 p-6 max-w-lg mx-auto">
      <div v-if="user" class="space-y-3 mb-8">
        <div class="flex justify-between text-sm">
          <span class="text-gray-500">Login</span>
          <span class="text-gray-900 font-medium">{{ user.login }}</span>
        </div>
        <div class="flex justify-between text-sm">
          <span class="text-gray-500">ID</span>
          <span class="text-gray-900">{{ user.id }}</span>
        </div>
        <div class="flex justify-between text-sm">
          <span class="text-gray-500">Created</span>
          <span class="text-gray-900">{{ user.createdAt }}</span>
        </div>
      </div>

      <h2 class="text-lg font-semibold text-gray-900 mb-4">Change Password</h2>
      <form @submit.prevent="handleChangePassword">
        <div class="mb-4">
          <label class="block text-sm font-medium text-gray-700 mb-1">New Password</label>
          <VInput
            v-model="newPassword"
            type="password"
            required
            minlength="4"
          />
        </div>
        <p v-if="error" class="text-sm text-red-600 mb-3">{{ error }}</p>
        <p v-if="success" class="text-sm text-green-600 mb-3">Password changed successfully</p>
        <button
          type="submit"
          :disabled="saving"
          class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 disabled:opacity-50"
        >Change Password</button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useAuth } from '../composables/useAuth'
import VInput from '../components/VInput.vue'

const { user, changePassword } = useAuth()

const newPassword = ref('')
const saving = ref(false)
const error = ref('')
const success = ref(false)

async function handleChangePassword() {
  saving.value = true
  error.value = ''
  success.value = false
  try {
    await changePassword(newPassword.value)
    newPassword.value = ''
    success.value = true
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Failed'
  } finally {
    saving.value = false
  }
}
</script>
