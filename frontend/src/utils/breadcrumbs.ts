import { reactive } from 'vue'

export interface BreadcrumbProject {
  id: number
  title: string
}

export interface BreadcrumbReview {
  id: number
  title: string
}

export const breadcrumbs = reactive<{
  project: BreadcrumbProject | null
  review: BreadcrumbReview | null
}>({
  project: null,
  review: null,
})

export function setProjectCrumb(id: number, title: string) {
  breadcrumbs.project = { id, title }
  breadcrumbs.review = null
}

export function setReviewCrumb(id: number, title: string) {
  breadcrumbs.review = { id, title }
}

export function clearCrumbs() {
  breadcrumbs.project = null
  breadcrumbs.review = null
}
