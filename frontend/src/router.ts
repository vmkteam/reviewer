import { createRouter, createWebHistory } from 'vue-router'
import ProjectsPage from './pages/ProjectsPage.vue'
import ReviewsPage from './pages/ReviewsPage.vue'
import ReviewPage from './pages/ReviewPage.vue'

const router = createRouter({
  history: createWebHistory('/reviews/'),
  routes: [
    { path: '/', name: 'projects', component: ProjectsPage },
    { path: '/project/:id/', name: 'reviews', component: ReviewsPage, props: true },
    { path: '/:id/', name: 'review', component: ReviewPage, props: true },
  ],
})

export default router
