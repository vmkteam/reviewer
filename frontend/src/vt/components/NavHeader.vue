<template>
  <header class="bg-surface border-b border-edge sticky top-0 z-50">
    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
      <div class="flex items-center h-14 justify-between">
        <div class="flex items-center gap-4 sm:gap-6">
          <router-link to="/projects" class="text-lg font-bold tracking-tight text-fg hover:text-accent transition-colors shrink-0">
            <span class="relative top-[-1px]">reviewer</span>
          </router-link>

          <nav class="flex items-center gap-3 text-sm font-medium shrink-0">
            <a href="/reviews/" class="text-fg-subtle hover:text-fg-secondary transition-colors">Reviews</a>
            <span class="text-accent">VT</span>
          </nav>

          <span class="h-5 w-px bg-edge hidden md:block"></span>

          <nav class="hidden md:flex items-center gap-4 text-sm font-medium">
            <router-link to="/projects" class="hover:text-fg transition-colors" active-class="!text-accent" :class="$route.path.startsWith('/projects') ? 'text-accent' : 'text-fg-secondary'">Projects</router-link>
            <router-link to="/prompts" class="hover:text-fg transition-colors" active-class="!text-accent" :class="$route.path.startsWith('/prompts') ? 'text-accent' : 'text-fg-secondary'">Prompts</router-link>
            <router-link to="/task-trackers" class="hover:text-fg transition-colors" active-class="!text-accent" :class="$route.path.startsWith('/task-trackers') ? 'text-accent' : 'text-fg-secondary'">Trackers</router-link>
            <router-link to="/slack-channels" class="hover:text-fg transition-colors" active-class="!text-accent" :class="$route.path.startsWith('/slack-channels') ? 'text-accent' : 'text-fg-secondary'">Slack</router-link>
            <router-link to="/users" class="hover:text-fg transition-colors" active-class="!text-accent" :class="$route.path.startsWith('/users') ? 'text-accent' : 'text-fg-secondary'">Users</router-link>
          </nav>
        </div>

        <div class="flex items-center gap-3 text-sm">
          <button @click="toggle" class="text-fg-subtle hover:text-fg-secondary transition-colors" :title="isDark ? 'Light mode' : 'Dark mode'">
            <svg v-if="!isDark" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
              <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
            </svg>
            <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
              <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
            </svg>
          </button>
          <router-link to="/profile" class="text-fg-secondary hover:text-fg transition-colors hidden sm:inline">
            {{ user?.login ?? '' }}
          </router-link>
          <button @click="handleLogout" class="text-fg-subtle hover:text-danger transition-colors hidden sm:inline">Logout</button>

          <!-- Mobile hamburger -->
          <button @click="mobileOpen = !mobileOpen" class="md:hidden p-1.5 text-fg-muted hover:text-fg-secondary">
            <svg xmlns="http://www.w3.org/2000/svg" class="h-6 w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2">
              <path v-if="!mobileOpen" stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h16M4 18h16" />
              <path v-else stroke-linecap="round" stroke-linejoin="round" d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      </div>

      <!-- Mobile menu -->
      <nav v-if="mobileOpen" class="md:hidden pb-4 border-t border-edge-light pt-3 space-y-1">
        <router-link
          v-for="link in navLinks" :key="link.to"
          :to="link.to"
          class="block px-3 py-2 rounded-lg text-sm font-medium transition-colors"
          :class="$route.path.startsWith(link.to) ? 'bg-accent-light text-accent' : 'text-fg-secondary hover:bg-surface-alt'"
          @click="mobileOpen = false"
        >{{ link.label }}</router-link>
        <div class="border-t border-edge-light mt-2 pt-2 flex items-center justify-between px-3">
          <router-link to="/profile" class="text-sm text-fg-secondary hover:text-fg" @click="mobileOpen = false">
            {{ user?.login ?? 'Profile' }}
          </router-link>
          <button @click="handleLogout" class="text-sm text-fg-subtle hover:text-danger">Logout</button>
        </div>
      </nav>
    </div>
  </header>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuth } from '../composables/useAuth'
import { useTheme } from '../../composables/useTheme'

const router = useRouter()
const route = useRoute()
const { user, logout } = useAuth()
const { isDark, toggle } = useTheme()
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
