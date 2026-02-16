import { send } from './vtClient'
import { factory } from './vt.generated'

// Re-export types from generated file
export type { IFieldError as FieldError, IViewOps as ViewOps, IStatus as Status } from './vt.generated'
export type { IProject as Project, IProjectSummary as ProjectSummary, IProjectSearch as ProjectSearch } from './vt.generated'
export type { IPrompt as Prompt, IPromptSummary as PromptSummary, IPromptSearch as PromptSearch } from './vt.generated'
export type { ISlackChannel as SlackChannel, ISlackChannelSummary as SlackChannelSummary, ISlackChannelSearch as SlackChannelSearch } from './vt.generated'
export type { ITaskTracker as TaskTracker, ITaskTrackerSummary as TaskTrackerSummary, ITaskTrackerSearch as TaskTrackerSearch } from './vt.generated'
export type { IUser as User, IUserSummary as UserSummary, IUserSearch as UserSearch, IUserProfile as UserProfile } from './vt.generated'

const generated = factory(send)

// Wrapper: adapt namespace names and method signatures for composables
const vtApi = {
  auth: {
    login: (login: string, password: string, remember: boolean) => generated.auth.login({ login, password, remember }),
    logout: () => generated.auth.logout(),
    profile: () => generated.auth.profile(),
    changePassword: (password: string) => generated.auth.changePassword({ password }),
    vfsAuthToken: () => generated.auth.vfsAuthToken(),
  },
  project: {
    count: (search?: any) => generated.project.count({ search }),
    get: (search?: any, viewOps?: any) => generated.project.get({ search, viewOps }),
    getByID: (id: number) => generated.project.getByID({ id }),
    add: (project: any) => generated.project.add({ project }),
    update: (project: any) => generated.project.update({ project }),
    delete: (id: number) => generated.project.delete({ id }),
    validate: (project: any) => generated.project.validate({ project }),
    gitlabCI: (targetBranch: string) => generated.project.gitlabCI({ targetBranch }),
  },
  prompt: {
    count: (search?: any) => generated.prompt.count({ search }),
    get: (search?: any, viewOps?: any) => generated.prompt.get({ search, viewOps }),
    getByID: (id: number) => generated.prompt.getByID({ id }),
    add: (prompt: any) => generated.prompt.add({ prompt }),
    update: (prompt: any) => generated.prompt.update({ prompt }),
    delete: (id: number) => generated.prompt.delete({ id }),
    validate: (prompt: any) => generated.prompt.validate({ prompt }),
  },
  slackChannel: {
    count: (search?: any) => generated.slackchannel.count({ search }),
    get: (search?: any, viewOps?: any) => generated.slackchannel.get({ search, viewOps }),
    getByID: (id: number) => generated.slackchannel.getByID({ id }),
    add: (slackChannel: any) => generated.slackchannel.add({ slackChannel }),
    update: (slackChannel: any) => generated.slackchannel.update({ slackChannel }),
    delete: (id: number) => generated.slackchannel.delete({ id }),
    validate: (slackChannel: any) => generated.slackchannel.validate({ slackChannel }),
  },
  taskTracker: {
    count: (search?: any) => generated.tasktracker.count({ search }),
    get: (search?: any, viewOps?: any) => generated.tasktracker.get({ search, viewOps }),
    getByID: (id: number) => generated.tasktracker.getByID({ id }),
    add: (taskTracker: any) => generated.tasktracker.add({ taskTracker }),
    update: (taskTracker: any) => generated.tasktracker.update({ taskTracker }),
    delete: (id: number) => generated.tasktracker.delete({ id }),
    validate: (taskTracker: any) => generated.tasktracker.validate({ taskTracker }),
  },
  user: {
    count: (search?: any) => generated.user.count({ search }),
    get: (search?: any, viewOps?: any) => generated.user.get({ search, viewOps }),
    getByID: (id: number) => generated.user.getByID({ id }),
    add: (user: any) => generated.user.add({ user }),
    update: (user: any) => generated.user.update({ user }),
    delete: (id: number) => generated.user.delete({ id }),
    validate: (user: any) => generated.user.validate({ user }),
  },
}

export default vtApi
