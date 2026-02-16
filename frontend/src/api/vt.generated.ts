// @ts-nocheck
/* Code generated from jsonrpc schema by rpcgen v2.5.x with typescript v1.0.0; DO NOT EDIT. */
/* eslint-disable */
export interface IAuthChangePasswordParams {
  password: string
}

export interface IAuthLoginParams {
  login: string,
  password: string,
  remember: boolean
}

export interface IFieldError {
  field: string,
  error: string,
  constraint?: IFieldErrorConstraint // Help with generating an error message.
}

export interface IFieldErrorConstraint {
  max: number, // Max value for field.
  min: number // Min value for field.
}

export interface IProject {
  id: number,
  title: string,
  vcsURL: string,
  language: string,
  projectKey: string,
  promptId: number,
  taskTrackerId?: number,
  slackChannelId?: number,
  statusId: number,
  prompt?: IPromptSummary,
  taskTracker?: ITaskTrackerSummary,
  slackChannel?: ISlackChannelSummary,
  status?: IStatus
}

export interface IProjectAddParams {
  project: IProject
}

export interface IProjectCountParams {
  search?: IProjectSearch
}

export interface IProjectDeleteParams {
  id: number
}

export interface IProjectGetByIDParams {
  id: number
}

export interface IProjectGetParams {
  search?: IProjectSearch,
  viewOps?: IViewOps
}

export interface IProjectGitlabCIParams {
  targetBranch: string
}

export interface IProjectSearch {
  id?: number,
  title?: string,
  vcsURL?: string,
  language?: string,
  projectKey?: string,
  promptId?: number,
  taskTrackerId?: number,
  slackChannelId?: number,
  statusId?: number,
  ids: Array<number>
}

export interface IProjectSummary {
  id: number,
  title: string,
  vcsURL: string,
  language: string,
  projectKey: string,
  promptId: number,
  taskTrackerId?: number,
  slackChannelId?: number,
  prompt?: IPromptSummary,
  taskTracker?: ITaskTrackerSummary,
  slackChannel?: ISlackChannelSummary,
  status?: IStatus
}

export interface IProjectUpdateParams {
  project: IProject
}

export interface IProjectValidateParams {
  project: IProject
}

export interface IPrompt {
  id: number,
  title: string,
  common: string,
  architecture: string,
  code: string,
  security: string,
  tests: string,
  statusId: number,
  status?: IStatus
}

export interface IPromptAddParams {
  prompt: IPrompt
}

export interface IPromptCountParams {
  search?: IPromptSearch
}

export interface IPromptDeleteParams {
  id: number
}

export interface IPromptGetByIDParams {
  id: number
}

export interface IPromptGetParams {
  search?: IPromptSearch,
  viewOps?: IViewOps
}

export interface IPromptSearch {
  id?: number,
  title?: string,
  common?: string,
  architecture?: string,
  code?: string,
  security?: string,
  tests?: string,
  statusId?: number,
  ids: Array<number>
}

export interface IPromptSummary {
  id: number,
  title: string,
  common: string,
  architecture: string,
  code: string,
  security: string,
  tests: string,
  status?: IStatus
}

export interface IPromptUpdateParams {
  prompt: IPrompt
}

export interface IPromptValidateParams {
  prompt: IPrompt
}

export interface ISlackChannel {
  id: number,
  title: string,
  channel: string,
  webhookURL: string,
  statusId: number,
  status?: IStatus
}

export interface ISlackChannelSearch {
  id?: number,
  title?: string,
  channel?: string,
  webhookURL?: string,
  statusId?: number,
  ids: Array<number>
}

export interface ISlackChannelSummary {
  id: number,
  title: string,
  channel: string,
  webhookURL: string,
  status?: IStatus
}

export interface ISlackchannelAddParams {
  slackChannel: ISlackChannel
}

export interface ISlackchannelCountParams {
  search?: ISlackChannelSearch
}

export interface ISlackchannelDeleteParams {
  id: number
}

export interface ISlackchannelGetByIDParams {
  id: number
}

export interface ISlackchannelGetParams {
  search?: ISlackChannelSearch,
  viewOps?: IViewOps
}

export interface ISlackchannelUpdateParams {
  slackChannel: ISlackChannel
}

export interface ISlackchannelValidateParams {
  slackChannel: ISlackChannel
}

export interface IStatus {
  id: number,
  alias: string,
  title: string
}

export interface ITaskTracker {
  id: number,
  title: string,
  authToken: string,
  fetchPrompt: string,
  statusId: number,
  status?: IStatus
}

export interface ITaskTrackerSearch {
  id?: number,
  title?: string,
  authToken?: string,
  fetchPrompt?: string,
  statusId?: number,
  ids: Array<number>
}

export interface ITaskTrackerSummary {
  id: number,
  title: string,
  authToken: string,
  fetchPrompt: string,
  status?: IStatus
}

export interface ITasktrackerAddParams {
  taskTracker: ITaskTracker
}

export interface ITasktrackerCountParams {
  search?: ITaskTrackerSearch
}

export interface ITasktrackerDeleteParams {
  id: number
}

export interface ITasktrackerGetByIDParams {
  id: number
}

export interface ITasktrackerGetParams {
  search?: ITaskTrackerSearch,
  viewOps?: IViewOps
}

export interface ITasktrackerUpdateParams {
  taskTracker: ITaskTracker
}

export interface ITasktrackerValidateParams {
  taskTracker: ITaskTracker
}

export interface IUser {
  id: number,
  createdAt: string,
  login: string,
  password: string,
  lastActivityAt?: string,
  statusId: number,
  status?: IStatus
}

export interface IUserAddParams {
  user: IUser
}

export interface IUserCountParams {
  search?: IUserSearch
}

export interface IUserDeleteParams {
  id: number
}

export interface IUserGetByIDParams {
  id: number
}

export interface IUserGetParams {
  search?: IUserSearch,
  viewOps?: IViewOps
}

export interface IUserProfile {
  id: number,
  createdAt: string,
  login: string,
  lastActivityAt?: string,
  statusId: number
}

export interface IUserSearch {
  id?: number,
  login?: string,
  statusId?: number,
  lastActivityAtFrom?: string,
  lastActivityAtTo?: string,
  ids: Array<number>,
  notId?: number
}

export interface IUserSummary {
  id: number,
  createdAt: string,
  login: string,
  lastActivityAt?: string,
  status?: IStatus
}

export interface IUserUpdateParams {
  user: IUser
}

export interface IUserValidateParams {
  user: IUser
}

export interface IViewOps {
  page: number, // page number, default - 1
  pageSize: number, // items count per page, max - 500
  sortColumn: string, // sort by column name
  sortDesc: boolean // descending sort
}

export class AuthChangePasswordParams implements IAuthChangePasswordParams {
  static entityName = "authchangepasswordparams";

  password: string = null;
}

export class AuthLoginParams implements IAuthLoginParams {
  static entityName = "authloginparams";

  login: string = null;
  password: string = null;
  remember: boolean = false;
}

export class FieldError implements IFieldError {
  static entityName = "fielderror";

  field: string = null;
  error: string = null;
  constraint?: IFieldErrorConstraint = null;
}

export class FieldErrorConstraint implements IFieldErrorConstraint {
  static entityName = "fielderrorconstraint";

  max: number = 0;
  min: number = 0;
}

export class Project implements IProject {
  static entityName = "project";

  id: number = 0;
  title: string = null;
  vcsURL: string = null;
  language: string = null;
  projectKey: string = null;
  promptId: number = 0;
  taskTrackerId?: number = 0;
  slackChannelId?: number = 0;
  statusId: number = 0;
  prompt?: IPromptSummary = null;
  taskTracker?: ITaskTrackerSummary = null;
  slackChannel?: ISlackChannelSummary = null;
  status?: IStatus = null;
}

export class ProjectAddParams implements IProjectAddParams {
  static entityName = "projectaddparams";

  project: IProject = null;
}

export class ProjectCountParams implements IProjectCountParams {
  static entityName = "projectcountparams";

  search?: IProjectSearch = null;
}

export class ProjectDeleteParams implements IProjectDeleteParams {
  static entityName = "projectdeleteparams";

  id: number = 0;
}

export class ProjectGetByIDParams implements IProjectGetByIDParams {
  static entityName = "projectgetbyidparams";

  id: number = 0;
}

export class ProjectGetParams implements IProjectGetParams {
  static entityName = "projectgetparams";

  search?: IProjectSearch = null;
  viewOps?: IViewOps = null;
}

export class ProjectGitlabCIParams implements IProjectGitlabCIParams {
  static entityName = "projectgitlabciparams";

  targetBranch: string = null;
}

export class ProjectSearch implements IProjectSearch {
  static entityName = "projectsearch";

  id?: number = 0;
  title?: string = "";
  vcsURL?: string = "";
  language?: string = "";
  projectKey?: string = "";
  promptId?: number = 0;
  taskTrackerId?: number = 0;
  slackChannelId?: number = 0;
  statusId?: number = 0;
  ids: Array<number> = [0];
}

export class ProjectSummary implements IProjectSummary {
  static entityName = "project";

  id: number = 0;
  title: string = null;
  vcsURL: string = null;
  language: string = null;
  projectKey: string = null;
  promptId: number = 0;
  taskTrackerId?: number = 0;
  slackChannelId?: number = 0;
  prompt?: IPromptSummary = null;
  taskTracker?: ITaskTrackerSummary = null;
  slackChannel?: ISlackChannelSummary = null;
  status?: IStatus = null;
}

export class ProjectUpdateParams implements IProjectUpdateParams {
  static entityName = "projectupdateparams";

  project: IProject = null;
}

export class ProjectValidateParams implements IProjectValidateParams {
  static entityName = "projectvalidateparams";

  project: IProject = null;
}

export class Prompt implements IPrompt {
  static entityName = "prompt";

  id: number = 0;
  title: string = null;
  common: string = null;
  architecture: string = null;
  code: string = null;
  security: string = null;
  tests: string = null;
  statusId: number = 0;
  status?: IStatus = null;
}

export class PromptAddParams implements IPromptAddParams {
  static entityName = "promptaddparams";

  prompt: IPrompt = null;
}

export class PromptCountParams implements IPromptCountParams {
  static entityName = "promptcountparams";

  search?: IPromptSearch = null;
}

export class PromptDeleteParams implements IPromptDeleteParams {
  static entityName = "promptdeleteparams";

  id: number = 0;
}

export class PromptGetByIDParams implements IPromptGetByIDParams {
  static entityName = "promptgetbyidparams";

  id: number = 0;
}

export class PromptGetParams implements IPromptGetParams {
  static entityName = "promptgetparams";

  search?: IPromptSearch = null;
  viewOps?: IViewOps = null;
}

export class PromptSearch implements IPromptSearch {
  static entityName = "promptsearch";

  id?: number = 0;
  title?: string = "";
  common?: string = "";
  architecture?: string = "";
  code?: string = "";
  security?: string = "";
  tests?: string = "";
  statusId?: number = 0;
  ids: Array<number> = [0];
}

export class PromptSummary implements IPromptSummary {
  static entityName = "prompt";

  id: number = 0;
  title: string = null;
  common: string = null;
  architecture: string = null;
  code: string = null;
  security: string = null;
  tests: string = null;
  status?: IStatus = null;
}

export class PromptUpdateParams implements IPromptUpdateParams {
  static entityName = "promptupdateparams";

  prompt: IPrompt = null;
}

export class PromptValidateParams implements IPromptValidateParams {
  static entityName = "promptvalidateparams";

  prompt: IPrompt = null;
}

export class SlackChannel implements ISlackChannel {
  static entityName = "slackchannel";

  id: number = 0;
  title: string = null;
  channel: string = null;
  webhookURL: string = null;
  statusId: number = 0;
  status?: IStatus = null;
}

export class SlackChannelSearch implements ISlackChannelSearch {
  static entityName = "slackchannelsearch";

  id?: number = 0;
  title?: string = "";
  channel?: string = "";
  webhookURL?: string = "";
  statusId?: number = 0;
  ids: Array<number> = [0];
}

export class SlackChannelSummary implements ISlackChannelSummary {
  static entityName = "slackchannel";

  id: number = 0;
  title: string = null;
  channel: string = null;
  webhookURL: string = null;
  status?: IStatus = null;
}

export class SlackchannelAddParams implements ISlackchannelAddParams {
  static entityName = "slackchanneladdparams";

  slackChannel: ISlackChannel = null;
}

export class SlackchannelCountParams implements ISlackchannelCountParams {
  static entityName = "slackchannelcountparams";

  search?: ISlackChannelSearch = null;
}

export class SlackchannelDeleteParams implements ISlackchannelDeleteParams {
  static entityName = "slackchanneldeleteparams";

  id: number = 0;
}

export class SlackchannelGetByIDParams implements ISlackchannelGetByIDParams {
  static entityName = "slackchannelgetbyidparams";

  id: number = 0;
}

export class SlackchannelGetParams implements ISlackchannelGetParams {
  static entityName = "slackchannelgetparams";

  search?: ISlackChannelSearch = null;
  viewOps?: IViewOps = null;
}

export class SlackchannelUpdateParams implements ISlackchannelUpdateParams {
  static entityName = "slackchannelupdateparams";

  slackChannel: ISlackChannel = null;
}

export class SlackchannelValidateParams implements ISlackchannelValidateParams {
  static entityName = "slackchannelvalidateparams";

  slackChannel: ISlackChannel = null;
}

export class Status implements IStatus {
  static entityName = "status";

  id: number = 0;
  alias: string = null;
  title: string = null;
}

export class TaskTracker implements ITaskTracker {
  static entityName = "tasktracker";

  id: number = 0;
  title: string = null;
  authToken: string = null;
  fetchPrompt: string = null;
  statusId: number = 0;
  status?: IStatus = null;
}

export class TaskTrackerSearch implements ITaskTrackerSearch {
  static entityName = "tasktrackersearch";

  id?: number = 0;
  title?: string = "";
  authToken?: string = "";
  fetchPrompt?: string = "";
  statusId?: number = 0;
  ids: Array<number> = [0];
}

export class TaskTrackerSummary implements ITaskTrackerSummary {
  static entityName = "tasktracker";

  id: number = 0;
  title: string = null;
  authToken: string = null;
  fetchPrompt: string = null;
  status?: IStatus = null;
}

export class TasktrackerAddParams implements ITasktrackerAddParams {
  static entityName = "tasktrackeraddparams";

  taskTracker: ITaskTracker = null;
}

export class TasktrackerCountParams implements ITasktrackerCountParams {
  static entityName = "tasktrackercountparams";

  search?: ITaskTrackerSearch = null;
}

export class TasktrackerDeleteParams implements ITasktrackerDeleteParams {
  static entityName = "tasktrackerdeleteparams";

  id: number = 0;
}

export class TasktrackerGetByIDParams implements ITasktrackerGetByIDParams {
  static entityName = "tasktrackergetbyidparams";

  id: number = 0;
}

export class TasktrackerGetParams implements ITasktrackerGetParams {
  static entityName = "tasktrackergetparams";

  search?: ITaskTrackerSearch = null;
  viewOps?: IViewOps = null;
}

export class TasktrackerUpdateParams implements ITasktrackerUpdateParams {
  static entityName = "tasktrackerupdateparams";

  taskTracker: ITaskTracker = null;
}

export class TasktrackerValidateParams implements ITasktrackerValidateParams {
  static entityName = "tasktrackervalidateparams";

  taskTracker: ITaskTracker = null;
}

export class User implements IUser {
  static entityName = "user";

  id: number = 0;
  createdAt: string = null;
  login: string = null;
  password: string = null;
  lastActivityAt?: string = null;
  statusId: number = 0;
  status?: IStatus = null;
}

export class UserAddParams implements IUserAddParams {
  static entityName = "useraddparams";

  user: IUser = null;
}

export class UserCountParams implements IUserCountParams {
  static entityName = "usercountparams";

  search?: IUserSearch = null;
}

export class UserDeleteParams implements IUserDeleteParams {
  static entityName = "userdeleteparams";

  id: number = 0;
}

export class UserGetByIDParams implements IUserGetByIDParams {
  static entityName = "usergetbyidparams";

  id: number = 0;
}

export class UserGetParams implements IUserGetParams {
  static entityName = "usergetparams";

  search?: IUserSearch = null;
  viewOps?: IViewOps = null;
}

export class UserProfile implements IUserProfile {
  static entityName = "userprofile";

  id: number = 0;
  createdAt: string = null;
  login: string = null;
  lastActivityAt?: string = null;
  statusId: number = 0;
}

export class UserSearch implements IUserSearch {
  static entityName = "usersearch";

  id?: number = 0;
  login?: string = "";
  statusId?: number = 0;
  lastActivityAtFrom?: string = "";
  lastActivityAtTo?: string = "";
  ids: Array<number> = [0];
  notId?: number = 0;
}

export class UserSummary implements IUserSummary {
  static entityName = "user";

  id: number = 0;
  createdAt: string = null;
  login: string = null;
  lastActivityAt?: string = null;
  status?: IStatus = null;
}

export class UserUpdateParams implements IUserUpdateParams {
  static entityName = "userupdateparams";

  user: IUser = null;
}

export class UserValidateParams implements IUserValidateParams {
  static entityName = "uservalidateparams";

  user: IUser = null;
}

export class ViewOps implements IViewOps {
  static entityName = "viewops";

  page: number = 1;
  pageSize: number = 25;
  sortColumn: string = "";
  sortDesc: boolean = false;
}

export const factory = (send: any) => ({
  auth: {
    /**
     * ChangePassword changes current user password.
     */
    changePassword(params: IAuthChangePasswordParams): Promise<string> {
      return send('auth.ChangePassword', params)
    },
    /**
     * Login authenticates user.
     */
    login(params: IAuthLoginParams): Promise<string> {
      return send('auth.Login', params)
    },
    /**
     * Logout current user from every session
     */
    logout(): Promise<boolean> {
      return send('auth.Logout')
    },
    /**
     * Profile is a function that returns current user profile
     */
    profile(): Promise<IUserProfile> {
      return send('auth.Profile')
    },
    /**
     * VfsAuthToken get auth token for VFS requests
     */
    vfsAuthToken(): Promise<string> {
      return send('auth.VfsAuthToken')
    }
  },
  project: {
    /**
     * Add adds a Project from the query.
     */
    add(params: IProjectAddParams): Promise<IProject> {
      return send('project.Add', params)
    },
    /**
     * Count returns count Projects according to conditions in search params.
     */
    count(params: IProjectCountParams): Promise<number> {
      return send('project.Count', params)
    },
    /**
     * Delete deletes the Project by its ID.
     */
    delete(params: IProjectDeleteParams): Promise<boolean> {
      return send('project.Delete', params)
    },
    /**
     * Get returns а list of Projects according to conditions in search params.
     */
    get(params: IProjectGetParams): Promise<Array<IProjectSummary>> {
      return send('project.Get', params)
    },
    /**
     * GetByID returns a Project by its ID.
     */
    getByID(params: IProjectGetByIDParams): Promise<IProject> {
      return send('project.GetByID', params)
    },
    /**
     * GitlabCI returns a generated GitLab CI YAML fragment.
     */
    gitlabCI(params: IProjectGitlabCIParams): Promise<string> {
      return send('project.GitlabCI', params)
    },
    /**
     * Update updates the Project data identified by id from the query.
     */
    update(params: IProjectUpdateParams): Promise<boolean> {
      return send('project.Update', params)
    },
    /**
     * Validate verifies that Project data is valid.
     */
    validate(params: IProjectValidateParams): Promise<Array<IFieldError>> {
      return send('project.Validate', params)
    }
  },
  prompt: {
    /**
     * Add adds a Prompt from the query.
     */
    add(params: IPromptAddParams): Promise<IPrompt> {
      return send('prompt.Add', params)
    },
    /**
     * Count returns count Prompts according to conditions in search params.
     */
    count(params: IPromptCountParams): Promise<number> {
      return send('prompt.Count', params)
    },
    /**
     * Delete deletes the Prompt by its ID.
     */
    delete(params: IPromptDeleteParams): Promise<boolean> {
      return send('prompt.Delete', params)
    },
    /**
     * Get returns а list of Prompts according to conditions in search params.
     */
    get(params: IPromptGetParams): Promise<Array<IPromptSummary>> {
      return send('prompt.Get', params)
    },
    /**
     * GetByID returns a Prompt by its ID.
     */
    getByID(params: IPromptGetByIDParams): Promise<IPrompt> {
      return send('prompt.GetByID', params)
    },
    /**
     * Update updates the Prompt data identified by id from the query.
     */
    update(params: IPromptUpdateParams): Promise<boolean> {
      return send('prompt.Update', params)
    },
    /**
     * Validate verifies that Prompt data is valid.
     */
    validate(params: IPromptValidateParams): Promise<Array<IFieldError>> {
      return send('prompt.Validate', params)
    }
  },
  slackchannel: {
    /**
     * Add adds a SlackChannel from the query.
     */
    add(params: ISlackchannelAddParams): Promise<ISlackChannel> {
      return send('slackchannel.Add', params)
    },
    /**
     * Count returns count SlackChannels according to conditions in search params.
     */
    count(params: ISlackchannelCountParams): Promise<number> {
      return send('slackchannel.Count', params)
    },
    /**
     * Delete deletes the SlackChannel by its ID.
     */
    delete(params: ISlackchannelDeleteParams): Promise<boolean> {
      return send('slackchannel.Delete', params)
    },
    /**
     * Get returns а list of SlackChannels according to conditions in search params.
     */
    get(params: ISlackchannelGetParams): Promise<Array<ISlackChannelSummary>> {
      return send('slackchannel.Get', params)
    },
    /**
     * GetByID returns a SlackChannel by its ID.
     */
    getByID(params: ISlackchannelGetByIDParams): Promise<ISlackChannel> {
      return send('slackchannel.GetByID', params)
    },
    /**
     * Update updates the SlackChannel data identified by id from the query.
     */
    update(params: ISlackchannelUpdateParams): Promise<boolean> {
      return send('slackchannel.Update', params)
    },
    /**
     * Validate verifies that SlackChannel data is valid.
     */
    validate(params: ISlackchannelValidateParams): Promise<Array<IFieldError>> {
      return send('slackchannel.Validate', params)
    }
  },
  tasktracker: {
    /**
     * Add adds a TaskTracker from the query.
     */
    add(params: ITasktrackerAddParams): Promise<ITaskTracker> {
      return send('tasktracker.Add', params)
    },
    /**
     * Count returns count TaskTrackers according to conditions in search params.
     */
    count(params: ITasktrackerCountParams): Promise<number> {
      return send('tasktracker.Count', params)
    },
    /**
     * Delete deletes the TaskTracker by its ID.
     */
    delete(params: ITasktrackerDeleteParams): Promise<boolean> {
      return send('tasktracker.Delete', params)
    },
    /**
     * Get returns а list of TaskTrackers according to conditions in search params.
     */
    get(params: ITasktrackerGetParams): Promise<Array<ITaskTrackerSummary>> {
      return send('tasktracker.Get', params)
    },
    /**
     * GetByID returns a TaskTracker by its ID.
     */
    getByID(params: ITasktrackerGetByIDParams): Promise<ITaskTracker> {
      return send('tasktracker.GetByID', params)
    },
    /**
     * Update updates the TaskTracker data identified by id from the query.
     */
    update(params: ITasktrackerUpdateParams): Promise<boolean> {
      return send('tasktracker.Update', params)
    },
    /**
     * Validate verifies that TaskTracker data is valid.
     */
    validate(params: ITasktrackerValidateParams): Promise<Array<IFieldError>> {
      return send('tasktracker.Validate', params)
    }
  },
  user: {
    /**
     * Add a User from the query
     */
    add(params: IUserAddParams): Promise<IUser> {
      return send('user.Add', params)
    },
    /**
     * Count Users according to conditions in search params
     */
    count(params: IUserCountParams): Promise<number> {
      return send('user.Count', params)
    },
    /**
     * Delete deletes the User by its ID.
     */
    delete(params: IUserDeleteParams): Promise<boolean> {
      return send('user.Delete', params)
    },
    /**
     * Get а list of Users according to conditions in search params
     */
    get(params: IUserGetParams): Promise<Array<IUserSummary>> {
      return send('user.Get', params)
    },
    /**
     * GetByID returns a User by its ID.
     */
    getByID(params: IUserGetByIDParams): Promise<IUser> {
      return send('user.GetByID', params)
    },
    /**
     * Update updates the User data identified by id from the query
     */
    update(params: IUserUpdateParams): Promise<boolean> {
      return send('user.Update', params)
    },
    /**
     * Validate Verifies that User data is valid.
     */
    validate(params: IUserValidateParams): Promise<Array<IFieldError>> {
      return send('user.Validate', params)
    }
  }
})
