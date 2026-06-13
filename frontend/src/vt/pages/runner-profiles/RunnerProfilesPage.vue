<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-fg">Runner Profiles</h1>
      <VButton size="sm" to="/runner-profiles/new">Add Profile</VButton>
    </div>

    <SearchBar>
      <div>
        <label class="block text-xs font-medium text-fg-muted mb-1">Title</label>
        <VInput v-model="search.title" @input="applySearch" type="text" placeholder="Search..." />
      </div>
      <div>
        <label class="block text-xs font-medium text-fg-muted mb-1">Runner</label>
        <VInput v-model="search.runner" @input="applySearch" type="text" placeholder="claude, opencode..." />
      </div>
      <div>
        <label class="block text-xs font-medium text-fg-muted mb-1">Status</label>
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
      @row-click="(item: any) => router.push(`/runner-profiles/${item.id}`)"
    >
      <template #cell-title="{ item }">
        <span class="font-medium text-fg">{{ (item as RunnerProfileSummary).title }}</span>
      </template>
      <template #cell-runner="{ item }">
        <span class="font-mono text-xs px-1.5 py-0.5 rounded bg-edge-light text-fg-secondary">{{ (item as RunnerProfileSummary).runner }}</span>
      </template>
      <template #cell-model="{ item }">
        <span class="text-fg-secondary">{{ (item as RunnerProfileSummary).model || '—' }}</span>
      </template>
      <template #cell-isDefault="{ item }">
        <span v-if="(item as RunnerProfileSummary).isDefault" class="text-xs px-1.5 py-0.5 rounded bg-accent-light text-accent">default</span>
      </template>
      <template #cell-status="{ item }">
        <StatusBadge :status-id="(item as RunnerProfileSummary).status?.id" />
      </template>
    </DataTable>

    <Pagination :page="viewOps.page" :page-size="viewOps.pageSize" :total="total" @update:page="setPage" />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type RunnerProfileSummary } from '../../../api/vt'
import { useCrud } from '../../composables/useCrud'
import DataTable from '../../components/DataTable.vue'
import Pagination from '../../components/Pagination.vue'
import SearchBar from '../../components/SearchBar.vue'
import VInput from '../../components/VInput.vue'
import VSelect from '../../components/VSelect.vue'
import StatusBadge from '../../components/StatusBadge.vue'
import VButton from '../../components/VButton.vue'

const router = useRouter()
const { items, total, loading, viewOps, search, load, setSort, setPage, applySearch } = useCrud(vtApi.runnerprofile, 'runnerProfileId')

const columns = [
  { key: 'id', label: 'ID', sortable: true, sortKey: 'runnerProfileId' },
  { key: 'title', label: 'Title', sortable: true },
  { key: 'runner', label: 'Runner', sortable: true },
  { key: 'model', label: 'Model', sortable: false },
  { key: 'isDefault', label: 'Default', sortable: true, sortKey: 'isDefault' },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'statusId' },
]

onMounted(load)
</script>
