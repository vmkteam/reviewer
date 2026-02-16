<template>
  <div class="min-h-screen bg-gray-50">
    <NavHeader v-if="isAuthenticated" />
    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import NavHeader from './components/NavHeader.vue'
import { useAuth } from './composables/useAuth'

const { isAuthenticated, loadProfile } = useAuth()

onMounted(() => {
  if (isAuthenticated.value) {
    loadProfile()
  }
})
</script>
