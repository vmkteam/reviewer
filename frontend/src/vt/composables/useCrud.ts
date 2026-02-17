import { ref, reactive, type Ref, shallowRef } from 'vue'
import type { ViewOps } from '../../api/vt'

interface CrudApi<T, S> {
  count: (params: { search?: S }) => Promise<number>
  get: (params: { search?: S, viewOps?: ViewOps }) => Promise<T[]>
}

export function useCrud<T, S>(api: CrudApi<T, S>) {
  const items: Ref<T[]> = shallowRef([])
  const total = ref(0)
  const loading = ref(false)
  const error = ref('')

  const viewOps = reactive<ViewOps>({
    page: 1,
    pageSize: 25,
    sortColumn: 'id',
    sortDesc: true,
  })

  const search = reactive<Record<string, unknown>>({})

  async function load() {
    loading.value = true
    error.value = ''
    try {
      const searchParams = Object.fromEntries(
        Object.entries(search).filter(([, v]) => v !== '' && v !== null && v !== undefined)
      ) as S
      const hasSearch = Object.keys(searchParams as Record<string, unknown>).length > 0

      const [count, list] = await Promise.all([
        api.count({ search: hasSearch ? searchParams : undefined }),
        api.get({ search: hasSearch ? searchParams : undefined, viewOps: { ...viewOps } }),
      ])
      total.value = count
      items.value = list ?? []
    } catch (e: unknown) {
      error.value = e instanceof Error ? e.message : 'Unknown error'
    } finally {
      loading.value = false
    }
  }

  function setSort(column: string) {
    if (viewOps.sortColumn === column) {
      viewOps.sortDesc = !viewOps.sortDesc
    } else {
      viewOps.sortColumn = column
      viewOps.sortDesc = false
    }
    viewOps.page = 1
    load()
  }

  function setPage(page: number) {
    viewOps.page = page
    load()
  }

  function resetSearch() {
    for (const key of Object.keys(search)) {
      search[key] = undefined
    }
    viewOps.page = 1
    load()
  }

  function applySearch() {
    viewOps.page = 1
    load()
  }

  return {
    items,
    total,
    loading,
    error,
    viewOps,
    search,
    load,
    setSort,
    setPage,
    resetSearch,
    applySearch,
  }
}
