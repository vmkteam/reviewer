import HttpRpcClient from './HttpRpcClient'
import { factory } from './vt.generated'
import { getAuthKey, setAuthKey } from './auth'

// Re-export types from generated file
export type { IFieldError as FieldError, IViewOps as ViewOps, IStatus as Status } from './vt.generated'
export type { IProject as Project, IProjectSummary as ProjectSummary, IProjectSearch as ProjectSearch } from './vt.generated'
export type { IPrompt as Prompt, IPromptSummary as PromptSummary, IPromptSearch as PromptSearch } from './vt.generated'
export type { ISlackChannel as SlackChannel, ISlackChannelSummary as SlackChannelSummary, ISlackChannelSearch as SlackChannelSearch } from './vt.generated'
export type { ITaskTracker as TaskTracker, ITaskTrackerSummary as TaskTrackerSummary, ITaskTrackerSearch as TaskTrackerSearch } from './vt.generated'
export type { IUser as User, IUserSummary as UserSummary, IUserSearch as UserSearch, IUserProfile as UserProfile } from './vt.generated'

export const client = new HttpRpcClient({ url: '/v1/vt/', isClient: true })

// Set auth token from localStorage
const authKey = getAuthKey()
if (authKey) {
  client.setHeader('Authorization2', authKey)
}

// Handle 401: reset token, redirect to login
client.onUnauthorized = () => {
  setAuthKey(null)
  window.location.href = '/vt/login'
}

export default factory(client.call)
