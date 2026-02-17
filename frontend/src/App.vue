<template>
  <div class="min-h-screen bg-gray-50">
    <header class="bg-white border-b border-gray-200 sticky top-0 z-50">
      <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
        <div class="flex items-center h-14">
          <router-link to="/" class="flex items-center gap-2 text-gray-900 hover:text-blue-600 transition-colors">
            <span class="text-lg font-bold tracking-tight relative top-[-1px]">reviewer</span>
          </router-link>

          <nav class="ml-6 flex items-center gap-3 text-sm font-medium">
            <span class="text-blue-600">Reviews</span>
            <a href="/vt/" class="text-gray-400 hover:text-gray-600 transition-colors">VT</a>
          </nav>

          <span class="mx-4 h-5 w-px bg-gray-200"></span>

          <!-- Breadcrumbs -->
          <nav class="flex items-center gap-1.5 text-sm text-gray-400 min-w-0">
            <router-link to="/" class="hover:text-gray-600 transition-colors flex-shrink-0">Projects</router-link>
            <template v-if="breadcrumbs.project">
              <span class="flex-shrink-0">/</span>
              <router-link
                :to="{ name: 'reviews', params: { id: breadcrumbs.project.id } }"
                class="hover:text-gray-600 transition-colors truncate max-w-[120px] sm:max-w-[200px]"
                :title="breadcrumbs.project.title"
              >{{ breadcrumbs.project.title }}</router-link>
            </template>
            <template v-if="breadcrumbs.review">
              <span class="flex-shrink-0">/</span>
              <span class="text-gray-600 truncate max-w-[150px] sm:max-w-[300px]" :title="`#${breadcrumbs.review.id} ${breadcrumbs.review.title}`">
                #{{ breadcrumbs.review.id }} {{ breadcrumbs.review.title }}
              </span>
            </template>
          </nav>

          <div class="ml-auto">
            <a :href="githubUrl" target="_blank" class="text-xs text-gray-400 hover:text-gray-500 transition-colors">
              {{ version }}
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
import api from './api/factory'

const { breadcrumbs } = useBreadcrumbs()

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
