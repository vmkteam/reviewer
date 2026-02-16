<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900">Slack Channels</h1>
      <router-link
        to="/slack-channels/new"
        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors shrink-0"
      >Add Slack Channel</router-link>
    </div>

    <SearchBar>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Title</label>
        <input v-model="search.title" @input="applySearch" type="text" placeholder="Search..." class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm" />
      </div>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Channel</label>
        <input v-model="search.channel" @input="applySearch" type="text" placeholder="Search..." class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm" />
      </div>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Status</label>
        <select v-model="search.statusId" @change="applySearch" class="w-full rounded-lg border border-gray-300 px-3 py-2 text-sm">
          <option :value="undefined">All</option>
          <option :value="1">Enabled</option>
          <option :value="2">Disabled</option>
        </select>
      </div>
    </SearchBar>

    <DataTable
      :columns="columns"
      :items="items"
      :loading="loading"
      :sort-column="viewOps.sortColumn"
      :sort-desc="viewOps.sortDesc"
      @sort="setSort"
      @row-click="(item: any) => router.push(`/slack-channels/${item.id}`)"
    >
      <template #cell-title="{ item }">
        <span class="font-medium text-gray-900">{{ (item as SlackChannelSummary).title }}</span>
      </template>
      <template #cell-status="{ item }">
        <span
          class="badge"
          :class="(item as SlackChannelSummary).status?.id === 1 ? 'bg-green-100 text-green-700' : 'bg-gray-100 text-gray-600'"
        >{{ (item as SlackChannelSummary).status?.id === 1 ? 'Enabled' : 'Disabled' }}</span>
      </template>
    </DataTable>

    <Pagination :page="viewOps.page" :page-size="viewOps.pageSize" :total="total" @update:page="setPage" />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type SlackChannelSummary } from '../../../api/vt'
import { useCrud } from '../../composables/useCrud'
import DataTable from '../../components/DataTable.vue'
import Pagination from '../../components/Pagination.vue'
import SearchBar from '../../components/SearchBar.vue'

const router = useRouter()
const { items, total, loading, viewOps, search, load, setSort, setPage, applySearch } = useCrud(vtApi.slackChannel)

const columns = [
  { key: 'id', label: 'ID', sortable: true },
  { key: 'title', label: 'Title', sortable: true },
  { key: 'channel', label: 'Channel', sortable: true },
  { key: 'status', label: 'Status', sortable: true },
]

onMounted(load)
</script>
