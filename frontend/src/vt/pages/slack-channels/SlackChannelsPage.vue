<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900">Slack Channels</h1>
      <VButton size="sm" to="/slack-channels/new">Add Slack Channel</VButton>
    </div>

    <SearchBar>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Title</label>
        <VInput v-model="search.title" @input="applySearch" type="text" placeholder="Search..." />
      </div>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Channel</label>
        <VInput v-model="search.channel" @input="applySearch" type="text" placeholder="Search..." />
      </div>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Status</label>
        <VSelect v-model="search.statusId" @change="applySearch">
          <option :value="undefined">All</option>
          <option :value="1">Enabled</option>
          <option :value="2">Disabled</option>
        </VSelect>
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
        <StatusBadge :status-id="(item as SlackChannelSummary).status?.id" />
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
import VInput from '../../components/VInput.vue'
import VSelect from '../../components/VSelect.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import VButton from '../../components/VButton.vue'

const router = useRouter()
const { items, total, loading, viewOps, search, load, setSort, setPage, applySearch } = useCrud(vtApi.slackchannel, 'slackChannelId')

const columns = [
  { key: 'id', label: 'ID', sortable: true, sortKey: 'slackChannelId' },
  { key: 'title', label: 'Title', sortable: true },
  { key: 'channel', label: 'Channel', sortable: true },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'statusId' },
]

onMounted(load)
</script>
