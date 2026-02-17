<template>
  <div>
    <div class="flex items-center justify-between mb-6 gap-4">
      <h1 class="text-xl sm:text-2xl font-bold text-gray-900">Users</h1>
      <router-link
        to="/users/new"
        class="px-4 py-2 bg-blue-600 text-white text-sm font-medium rounded-lg hover:bg-blue-700 transition-colors shrink-0"
      >Add User</router-link>
    </div>

    <SearchBar>
      <div>
        <label class="block text-xs font-medium text-gray-500 mb-1">Login</label>
        <VInput v-model="search.login" @input="applySearch" type="text" placeholder="Search..." />
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
      @row-click="(item: any) => router.push(`/users/${item.id}`)"
    >
      <template #cell-login="{ item }">
        <span class="font-medium text-gray-900">{{ (item as UserSummary).login }}</span>
      </template>
      <template #cell-lastActivityAt="{ item }">
        {{ (item as UserSummary).lastActivityAt ?? '—' }}
      </template>
      <template #cell-status="{ item }">
        <StatusBadge :status-id="(item as UserSummary).status?.id" />
      </template>
    </DataTable>

    <Pagination :page="viewOps.page" :page-size="viewOps.pageSize" :total="total" @update:page="setPage" />
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import vtApi, { type UserSummary } from '../../../api/vt'
import { useCrud } from '../../composables/useCrud'
import DataTable from '../../components/DataTable.vue'
import Pagination from '../../components/Pagination.vue'
import SearchBar from '../../components/SearchBar.vue'
import VInput from '../../components/VInput.vue'
import VSelect from '../../components/VSelect.vue'
import StatusBadge from '../../components/StatusBadge.vue'

const router = useRouter()
const { items, total, loading, viewOps, search, load, setSort, setPage, applySearch } = useCrud(vtApi.user, 'userId')

const columns = [
  { key: 'id', label: 'ID', sortable: true, sortKey: 'userId' },
  { key: 'login', label: 'Login', sortable: true },
  { key: 'lastActivityAt', label: 'Last Activity' },
  { key: 'status', label: 'Status', sortable: true, sortKey: 'statusId' },
]

onMounted(load)
</script>
