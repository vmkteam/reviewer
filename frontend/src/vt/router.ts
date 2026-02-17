import { createRouter, createWebHistory } from 'vue-router'
import { getAuthKey } from '../api/auth'

import LoginPage from './pages/LoginPage.vue'
import ProfilePage from './pages/ProfilePage.vue'
import PromptsPage from './pages/prompts/PromptsPage.vue'
import PromptFormPage from './pages/prompts/PromptFormPage.vue'
import TaskTrackersPage from './pages/task-trackers/TaskTrackersPage.vue'
import TaskTrackerFormPage from './pages/task-trackers/TaskTrackerFormPage.vue'
import SlackChannelsPage from './pages/slack-channels/SlackChannelsPage.vue'
import SlackChannelFormPage from './pages/slack-channels/SlackChannelFormPage.vue'
import ProjectsPage from './pages/projects/ProjectsPage.vue'
import ProjectFormPage from './pages/projects/ProjectFormPage.vue'
import ProjectBulkAddPage from './pages/projects/ProjectBulkAddPage.vue'
import UsersPage from './pages/users/UsersPage.vue'
import UserFormPage from './pages/users/UserFormPage.vue'

const router = createRouter({
  history: createWebHistory('/vt/'),
  routes: [
    { path: '/login', name: 'login', component: LoginPage, meta: { public: true } },
    { path: '/', redirect: '/projects' },
    { path: '/profile', name: 'profile', component: ProfilePage },
    { path: '/prompts', name: 'prompts', component: PromptsPage },
    { path: '/prompts/new', name: 'prompt-new', component: PromptFormPage },
    { path: '/prompts/:id', name: 'prompt-edit', component: PromptFormPage, props: true },
    { path: '/task-trackers', name: 'task-trackers', component: TaskTrackersPage },
    { path: '/task-trackers/new', name: 'task-tracker-new', component: TaskTrackerFormPage },
    { path: '/task-trackers/:id', name: 'task-tracker-edit', component: TaskTrackerFormPage, props: true },
    { path: '/slack-channels', name: 'slack-channels', component: SlackChannelsPage },
    { path: '/slack-channels/new', name: 'slack-channel-new', component: SlackChannelFormPage },
    { path: '/slack-channels/:id', name: 'slack-channel-edit', component: SlackChannelFormPage, props: true },
    { path: '/projects', name: 'projects', component: ProjectsPage },
    { path: '/projects/new', name: 'project-new', component: ProjectFormPage },
    { path: '/projects/bulk-add', name: 'project-bulk-add', component: ProjectBulkAddPage },
    { path: '/projects/:id', name: 'project-edit', component: ProjectFormPage, props: true },
    { path: '/users', name: 'users', component: UsersPage },
    { path: '/users/new', name: 'user-new', component: UserFormPage },
    { path: '/users/:id', name: 'user-edit', component: UserFormPage, props: true },
  ],
})

router.beforeEach((to) => {
  if (!to.meta.public && !getAuthKey()) {
    return { name: 'login' }
  }
})

export default router
