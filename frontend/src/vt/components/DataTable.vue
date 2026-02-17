<template>
  <div class="bg-white rounded-lg border border-gray-200 overflow-hidden">
    <div class="overflow-x-auto">
      <table class="min-w-full divide-y divide-gray-200">
        <thead class="bg-gray-50">
          <tr>
            <th
              v-for="col in columns"
              :key="col.key"
              class="px-3 sm:px-4 py-3 text-left text-xs font-medium text-gray-500 uppercase tracking-wider"
              :class="{ 'cursor-pointer hover:text-gray-700 select-none': col.sortable }"
              @click="col.sortable && $emit('sort', col.sortKey ?? col.key)"
            >
              <div class="flex items-center gap-1">
                {{ col.label }}
                <template v-if="col.sortable && sortColumn === (col.sortKey ?? col.key)">
                  <span class="text-blue-600">{{ sortDesc ? '▼' : '▲' }}</span>
                </template>
              </div>
            </th>
          </tr>
        </thead>
        <tbody class="divide-y divide-gray-200">
          <tr v-if="loading" class="text-center">
            <td :colspan="columns.length" class="px-3 sm:px-4 py-8">
              <div class="flex justify-center"><div class="spinner"></div></div>
            </td>
          </tr>
          <tr v-else-if="items.length === 0" class="text-center">
            <td :colspan="columns.length" class="px-3 sm:px-4 py-8 text-gray-400 text-sm">No data</td>
          </tr>
          <tr
            v-else
            v-for="item in items"
            :key="(item as any).id"
            class="row-hover hover:bg-gray-50 cursor-pointer"
            @click="$emit('row-click', item)"
          >
            <td v-for="col in columns" :key="col.key" class="px-3 sm:px-4 py-3 text-sm text-gray-700 whitespace-nowrap">
              <slot :name="`cell-${col.key}`" :item="item" :value="(item as any)[col.key]">
                {{ (item as any)[col.key] }}
              </slot>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
export interface Column {
  key: string
  label: string
  sortable?: boolean
  sortKey?: string
}

defineProps<{
  columns: Column[]
  items: unknown[]
  loading?: boolean
  sortColumn?: string
  sortDesc?: boolean
}>()

defineEmits<{
  sort: [column: string]
  'row-click': [item: unknown]
}>()
</script>
