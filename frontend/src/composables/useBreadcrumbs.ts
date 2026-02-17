import { reactive } from 'vue'

export interface BreadcrumbProject {
  id: number
  title: string
}

export interface BreadcrumbReview {
  id: number
  title: string
}

const breadcrumbs = reactive<{
  project: BreadcrumbProject | null
  review: BreadcrumbReview | null
}>({
  project: null,
  review: null,
})

export function useBreadcrumbs() {
  function setProject(id: number, title: string) {
    breadcrumbs.project = { id, title }
    breadcrumbs.review = null
  }

  function setReview(id: number, title: string) {
    breadcrumbs.review = { id, title }
  }

  function clear() {
    breadcrumbs.project = null
    breadcrumbs.review = null
  }

  return {
    breadcrumbs,
    setProject,
    setReview,
    clear,
  }
}
