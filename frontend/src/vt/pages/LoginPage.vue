<template>
  <div class="min-h-screen flex items-center justify-center">
    <div class="bg-white rounded-xl shadow-sm border border-gray-200 p-8 w-full max-w-sm">
      <h1 class="text-2xl font-bold text-gray-900 mb-6 text-center">VT Admin</h1>
      <form @submit.prevent="handleLogin">
        <div class="mb-4">
          <label class="block text-sm font-medium text-gray-700 mb-1">Login</label>
          <VInput
            v-model="login"
            type="text"
            required
            placeholder="Enter login"
            autofocus
          />
        </div>
        <div class="mb-4">
          <label class="block text-sm font-medium text-gray-700 mb-1">Password</label>
          <VInput
            v-model="password"
            type="password"
            required
            placeholder="Enter password"
          />
        </div>
        <div class="mb-6">
          <label class="flex items-center gap-2 cursor-pointer">
            <input v-model="remember" type="checkbox" class="accent-blue-600" />
            <span class="text-sm text-gray-600">Remember me</span>
          </label>
        </div>
        <p v-if="error" class="text-sm text-red-600 mb-4">{{ error }}</p>
        <button
          type="submit"
          :disabled="loading"
          class="w-full py-2.5 bg-blue-600 text-white font-medium rounded-lg hover:bg-blue-700 transition-colors disabled:opacity-50"
        >
          <span v-if="loading" class="inline-block spinner w-4 h-4 border-2 border-white/30 border-t-white"></span>
          <span v-else>Sign In</span>
        </button>
      </form>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import VInput from '../components/VInput.vue'

const router = useRouter()
const { login: doLogin } = useAuth()

const login = ref('')
const password = ref('')
const remember = ref(false)
const loading = ref(false)
const error = ref('')

async function handleLogin() {
  loading.value = true
  error.value = ''
  try {
    await doLogin(login.value, password.value, remember.value)
    router.push('/projects')
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : 'Login failed'
  } finally {
    loading.value = false
  }
}
</script>
