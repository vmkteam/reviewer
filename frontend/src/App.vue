<template>
  <div class="min-h-screen bg-surface-alt">
    <header class="bg-surface border-b border-edge sticky top-0 z-50">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center h-14">
          <router-link to="/" class="flex items-center gap-2 text-fg hover:text-accent transition-colors">
            <span class="text-lg font-bold tracking-tight relative top-[-1px]">reviewer</span>
          </router-link>

          <nav class="ml-6 flex items-center gap-3 text-sm font-medium">
            <span class="text-accent">Reviews</span>
            <a href="/vt/" class="text-fg-subtle hover:text-fg-secondary transition-colors">VT</a>
          </nav>

          <span class="mx-2 sm:mx-4 h-5 w-px bg-edge"></span>

          <!-- Breadcrumbs -->
          <nav class="flex items-center gap-1.5 text-sm text-fg-subtle min-w-0">
            <router-link to="/" class="hover:text-fg-secondary transition-colors flex-shrink-0">Projects</router-link>
            <template v-if="breadcrumbs.project">
              <span class="flex-shrink-0">/</span>
              <router-link
                :to="{ name: 'reviews', params: { id: breadcrumbs.project.id } }"
                class="hover:text-fg-secondary transition-colors truncate max-w-[120px] sm:max-w-[200px]"
                :title="breadcrumbs.project.title"
              >{{ breadcrumbs.project.title }}</router-link>
            </template>
            <template v-if="breadcrumbs.review">
              <span class="flex-shrink-0">/</span>
              <span class="text-fg-secondary truncate max-w-[150px] sm:max-w-[300px]" :title="`#${breadcrumbs.review.id} ${breadcrumbs.review.title}`">
                #{{ breadcrumbs.review.id }} {{ breadcrumbs.review.title }}
              </span>
            </template>
          </nav>

          <div class="ml-auto flex items-center gap-3">
            <button @click="toggle" class="text-fg-subtle hover:text-fg-secondary transition-colors" :title="isDark ? 'Light mode' : 'Dark mode'">
              <!-- Moon icon (shown in light mode) -->
              <svg v-if="!isDark" xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
              </svg>
              <!-- Sun icon (shown in dark mode) -->
              <svg v-else xmlns="http://www.w3.org/2000/svg" class="h-4 w-4" viewBox="0 0 20 20" fill="currentColor">
                <path fill-rule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clip-rule="evenodd" />
              </svg>
            </button>
            <a :href="githubUrl" target="_blank" class="text-fg-subtle hover:text-fg-secondary transition-colors" :title="version ? `version: ${version}` : ''">
              <svg class="h-5 w-5" viewBox="0 0 16 16" fill="currentColor">
                <path d="M8 0c4.42 0 8 3.58 8 8a8.013 8.013 0 0 1-5.45 7.59c-.4.08-.55-.17-.55-.38 0-.27.01-1.13.01-2.2 0-.75-.25-1.23-.54-1.48 1.78-.2 3.65-.88 3.65-3.95 0-.88-.31-1.59-.82-2.15.08-.2.36-1.02-.08-2.12 0 0-.67-.22-2.2.82-.64-.18-1.32-.27-2-.27-.68 0-1.36.09-2 .27-1.53-1.03-2.2-.82-2.2-.82-.44 1.1-.16 1.92-.08 2.12-.51.56-.82 1.28-.82 2.15 0 3.06 1.86 3.75 3.64 3.95-.23.2-.44.55-.51 1.07-.46.21-1.61.55-2.33-.66-.15-.24-.6-.83-1.23-.82-.67.01-.27.38.01.53.34.19.73.9.82 1.13.16.45.68 1.31 2.69.94 0 .67.01 1.3.01 1.49 0 .21-.15.45-.55.38A7.995 7.995 0 0 1 0 8c0-4.42 3.58-8 8-8Z" />
              </svg>
            </a>
          </div>
        </div>
      </div>
    </header>

    <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <router-view />
    </main>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { useBreadcrumbs } from './composables/useBreadcrumbs'
import { useTheme } from './composables/useTheme'
import api from './api/factory'

const { breadcrumbs } = useBreadcrumbs()
const { isDark, toggle } = useTheme()

const githubUrl = 'https://github.com/vmkteam/reviewer'
const version = ref('')

onMounted(async () => {
  try {
    version.value = await api.app.version()
  } catch {
    // ignore
  }
})
</script>
