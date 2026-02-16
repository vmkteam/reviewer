<template>
  <header class="bg-white border-b border-gray-200 sticky top-0 z-50">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center h-14 justify-between">
        <div class="flex items-center gap-4 sm:gap-6">
          <router-link to="/projects" class="text-lg font-bold tracking-tight text-gray-900 hover:text-blue-600 transition-colors shrink-0">
            <span class="relative top-[-1px]">reviewer</span>
          </router-link>

          <nav class="flex items-center gap-3 text-sm font-medium shrink-0">
            <a href="/reviews/" class="text-gray-400 hover:text-gray-600 transition-colors">Reviews</a>
            <span class="text-blue-600">VT</span>
          </nav>

          <span class="h-5 w-px bg-gray-200 hidden md:block"></span>

          <nav class="hidden md:flex items-center gap-4 text-sm font-medium">
            <router-link to="/projects" class="hover:text-gray-900 transition-colors" active-class="!text-blue-600" :class="$route.path.startsWith('/projects') ? 'text-blue-600' : 'text-gray-600'">Projects</router-link>
            <router-link to="/prompts" class="hover:text-gray-900 transition-colors" active-class="!text-blue-600" :class="$route.path.startsWith('/prompts') ? 'text-blue-600' : 'text-gray-600'">Prompts</router-link>
            <router-link to="/task-trackers" class="hover:text-gray-900 transition-colors" active-class="!text-blue-600" :class="$route.path.startsWith('/task-trackers') ? 'text-blue-600' : 'text-gray-600'">Trackers</router-link>
            <router-link to="/slack-channels" class="hover:text-gray-900 transition-colors" active-class="!text-blue-600" :class="$route.path.startsWith('/slack-channels') ? 'text-blue-600' : 'text-gray-600'">Slack</router-link>
            <router-link to="/users" class="hover:text-gray-900 transition-colors" active-class="!text-blue-600" :class="$route.path.startsWith('/users') ? 'text-blue-600' : 'text-gray-600'">Users</router-link>
          </nav>
        </div>

        <div class="flex items-center gap-3 text-sm">
          <router-link to="/profile" class="text-gray-600 hover:text-gray-900 transition-colors hidden sm:inline">
            {{ user?.login ?? '' }}
          </router-link>
          <button @click="handleLogout" class="text-gray-400 hover:text-red-600 transition-colors hidden sm:inline">Logout</button>

          <!-- Mobile hamburger -->
          <button @click="mobileOpen = !mobileOpen" class="md:hidden p-1.5 text-gray-500 hover:text-gray-700">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path v-if="!mobileOpen" stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
              <path v-else stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Mobile menu -->
      <nav v-if="mobileOpen" class="md:hidden pb-4 border-t border-gray-100 pt-3 space-y-1">
        <router-link
          v-for="link in navLinks" :key="link.to"
          :to="link.to"
          class="block px-3 py-2 rounded-lg text-sm font-medium transition-colors"
          :class="$route.path.startsWith(link.to) ? 'bg-blue-50 text-blue-700' : 'text-gray-600 hover:bg-gray-50'"
          @click="mobileOpen = false"
        >{{ link.label }}</router-link>
        <div class="border-t border-gray-100 mt-2 pt-2 flex items-center justify-between px-3">
          <router-link to="/profile" class="text-sm text-gray-600 hover:text-gray-900" @click="mobileOpen = false">
            {{ user?.login ?? 'Profile' }}
          </router-link>
          <button @click="handleLogout" class="text-sm text-gray-400 hover:text-red-600">Logout</button>
        </div>
      </nav>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '../composables/useAuth'

const router = useRouter()
const route = useRoute()
const { user, logout } = useAuth()
const mobileOpen = ref(false)

const navLinks = [
  { to: '/projects', label: 'Projects' },
  { to: '/prompts', label: 'Prompts' },
  { to: '/task-trackers', label: 'Task Trackers' },
  { to: '/slack-channels', label: 'Slack Channels' },
  { to: '/users', label: 'Users' },
]

watch(() => route.path, () => { mobileOpen.value = false })

async function handleLogout() {
  mobileOpen.value = false
  await logout()
  router.push({ name: 'login' })
}
</script>
