<template>
  <div class="min-h-screen flex items-center justify-center">
    <div class="bg-surface rounded-xl shadow-sm border border-edge p-6 sm:p-8 w-full max-w-sm mx-4 sm:mx-0">
      <h1 class="text-2xl font-bold text-fg mb-6 text-center">VT Admin</h1>
      <form @submit.prevent="handleLogin">
        <div class="mb-4">
          <label class="block text-sm font-medium text-fg-secondary mb-1">Login</label>
          <VInput
            v-model="login"
            type="text"
            required
            placeholder="Enter login"
            autofocus
          />
        </div>
        <div class="mb-4">
          <label class="block text-sm font-medium text-fg-secondary mb-1">Password</label>
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
            <span class="text-sm text-fg-secondary">Remember me</span>
          </label>
        </div>
        <p v-if="error" class="text-sm text-danger mb-4">{{ error }}</p>
        <button
          type="submit"
          :disabled="loading"
          class="w-full py-2.5 bg-accent text-white font-medium rounded-lg hover:bg-accent-hover transition-colors disabled:opacity-50"
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
