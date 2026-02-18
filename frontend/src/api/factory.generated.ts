// @ts-nocheck
/* Code generated from jsonrpc schema by rpcgen v2.5.x with typescript v1.0.0; DO NOT EDIT. */
/* eslint-disable */
export interface IIssue {
  issueId: number,
  reviewId: number,
  title: string,
  severity: string,
  description: string,
  content: string,
  file: string,
  lines: string,
  issueType: string,
  reviewType: string,
  commitHash: string,
  isFalsePositive?: boolean,
  comment?: string
}

export interface IIssueFilters {
  severity?: string,
  issueType?: string,
  reviewType?: string,
  isFalsePositive?: boolean
}

export interface IIssueStats {
  critical: number,
  high: number,
  medium: number,
  low: number,
  total: number
}

export interface ILastReview {
  createdAt: string,
  author: string,
  trafficLight: string
}

export interface IModelInfo {
  model: string,
  inputTokens: number,
  outputTokens: number,
  costUsd: number
}

export interface IProject {
  projectId: number,
  title: string,
  vcsURL: string,
  language: string,
  createdAt: string,
  reviewCount: number,
  lastReview?: ILastReview
}

export interface IReview {
  reviewId: number,
  projectId: number,
  title: string,
  description: string,
  externalId: string,
  trafficLight: string,
  commitHash: string,
  sourceBranch: string,
  targetBranch: string,
  author: string,
  createdAt: string,
  durationMs: number,
  modelInfo: IModelInfo,
  reviewFiles: Array<IReviewFile>,
  lastVersionReviewId?: number
}

export interface IReviewCountIssuesByProjectParams {
  projectId: number,
  filters?: IIssueFilters
}

export interface IReviewCountIssuesParams {
  reviewId: number,
  filters?: IIssueFilters
}

export interface IReviewCountParams {
  projectId: number,
  filters?: IReviewFilters
}

export interface IReviewFeedbackParams {
  issueId: number,
  isFalsePositive?: boolean
}

export interface IReviewFile {
  reviewFileId: number,
  reviewType: string,
  trafficLight: string,
  summary: string,
  issueStats: IIssueStats,
  content: string
}

export interface IReviewFileSummary {
  reviewType: string,
  trafficLight: string,
  issueStats: IIssueStats
}

export interface IReviewFilters {
  author?: string,
  trafficLight?: string
}

export interface IReviewGetByIDParams {
  reviewId: number
}

export interface IReviewGetParams {
  projectId: number,
  filters?: IReviewFilters,
  fromReviewId?: number
}

export interface IReviewIssuesByProjectParams {
  projectId: number,
  filters?: IIssueFilters,
  fromIssueId?: number
}

export interface IReviewIssuesParams {
  reviewId: number,
  filters?: IIssueFilters
}

export interface IReviewSetCommentParams {
  issueId: number,
  comment?: string
}

export interface IReviewSummary {
  reviewId: number,
  title: string,
  externalId: string,
  trafficLight: string,
  author: string,
  sourceBranch: string,
  targetBranch: string,
  createdAt: string,
  reviewFiles: Array<IReviewFileSummary>,
  lastVersionReviewId?: number
}

export class Issue implements IIssue {
  static entityName = "issue";

  issueId: number = 0;
  reviewId: number = 0;
  title: string = null;
  severity: string = null;
  description: string = null;
  content: string = null;
  file: string = null;
  lines: string = null;
  issueType: string = null;
  reviewType: string = null;
  commitHash: string = null;
  isFalsePositive?: boolean = false;
  comment?: string = null;
}

export class IssueFilters implements IIssueFilters {
  static entityName = "issuefilters";

  severity?: string = null;
  issueType?: string = null;
  reviewType?: string = null;
  isFalsePositive?: boolean = false;
}

export class IssueStats implements IIssueStats {
  static entityName = "issuestats";

  critical: number = 0;
  high: number = 0;
  medium: number = 0;
  low: number = 0;
  total: number = 0;
}

export class LastReview implements ILastReview {
  static entityName = "lastreview";

  createdAt: string = null;
  author: string = null;
  trafficLight: string = null;
}

export class ModelInfo implements IModelInfo {
  static entityName = "modelinfo";

  model: string = null;
  inputTokens: number = 0;
  outputTokens: number = 0;
  costUsd: number = 0;
}

export class Project implements IProject {
  static entityName = "project";

  projectId: number = 0;
  title: string = null;
  vcsURL: string = null;
  language: string = null;
  createdAt: string = null;
  reviewCount: number = 0;
  lastReview?: ILastReview = null;
}

export class Review implements IReview {
  static entityName = "review";

  reviewId: number = 0;
  projectId: number = 0;
  title: string = null;
  description: string = null;
  externalId: string = null;
  trafficLight: string = null;
  commitHash: string = null;
  sourceBranch: string = null;
  targetBranch: string = null;
  author: string = null;
  createdAt: string = null;
  durationMs: number = 0;
  modelInfo: IModelInfo = null;
  reviewFiles: Array<IReviewFile> = null;
  lastVersionReviewId?: number = 0;
}

export class ReviewCountIssuesByProjectParams implements IReviewCountIssuesByProjectParams {
  static entityName = "reviewcountissuesbyprojectparams";

  projectId: number = 0;
  filters?: IIssueFilters = null;
}

export class ReviewCountIssuesParams implements IReviewCountIssuesParams {
  static entityName = "reviewcountissuesparams";

  reviewId: number = 0;
  filters?: IIssueFilters = null;
}

export class ReviewCountParams implements IReviewCountParams {
  static entityName = "reviewcountparams";

  projectId: number = 0;
  filters?: IReviewFilters = null;
}

export class ReviewFeedbackParams implements IReviewFeedbackParams {
  static entityName = "reviewfeedbackparams";

  issueId: number = 0;
  isFalsePositive?: boolean = false;
}

export class ReviewFile implements IReviewFile {
  static entityName = "reviewfile";

  reviewFileId: number = 0;
  reviewType: string = null;
  trafficLight: string = null;
  summary: string = null;
  issueStats: IIssueStats = null;
  content: string = null;
}

export class ReviewFileSummary implements IReviewFileSummary {
  static entityName = "reviewfile";

  reviewType: string = null;
  trafficLight: string = null;
  issueStats: IIssueStats = null;
}

export class ReviewFilters implements IReviewFilters {
  static entityName = "reviewfilters";

  author?: string = null;
  trafficLight?: string = null;
}

export class ReviewGetByIDParams implements IReviewGetByIDParams {
  static entityName = "reviewgetbyidparams";

  reviewId: number = 0;
}

export class ReviewGetParams implements IReviewGetParams {
  static entityName = "reviewgetparams";

  projectId: number = 0;
  filters?: IReviewFilters = null;
  fromReviewId?: number = 0;
}

export class ReviewIssuesByProjectParams implements IReviewIssuesByProjectParams {
  static entityName = "reviewissuesbyprojectparams";

  projectId: number = 0;
  filters?: IIssueFilters = null;
  fromIssueId?: number = 0;
}

export class ReviewIssuesParams implements IReviewIssuesParams {
  static entityName = "reviewissuesparams";

  reviewId: number = 0;
  filters?: IIssueFilters = null;
}

export class ReviewSetCommentParams implements IReviewSetCommentParams {
  static entityName = "reviewsetcommentparams";

  issueId: number = 0;
  comment?: string = null;
}

export class ReviewSummary implements IReviewSummary {
  static entityName = "review";

  reviewId: number = 0;
  title: string = null;
  externalId: string = null;
  trafficLight: string = null;
  author: string = null;
  sourceBranch: string = null;
  targetBranch: string = null;
  createdAt: string = null;
  reviewFiles: Array<IReviewFileSummary> = null;
  lastVersionReviewId?: number = 0;
}

export const factory = (send: any) => ({
  app: {
    /**
     * Version returns application version.
     */
    version(): Promise<string> {
      return send('app.Version')
    }
  },
  review: {
    /**
     * Count returns count of reviews for a project.
     */
    count(params: IReviewCountParams): Promise<number> {
      return send('review.Count', params)
    },
    /**
     * CountIssues returns count of issues for a review.
     */
    countIssues(params: IReviewCountIssuesParams): Promise<number> {
      return send('review.CountIssues', params)
    },
    /**
     * CountIssuesByProject returns count of issues for a project.
     */
    countIssuesByProject(params: IReviewCountIssuesByProjectParams): Promise<number> {
      return send('review.CountIssuesByProject', params)
    },
    /**
     * Feedback updates false positive flag for an issue.
     */
    feedback(params: IReviewFeedbackParams): Promise<boolean> {
      return send('review.Feedback', params)
    },
    /**
     * Get returns list of reviews for a project.
     */
    get(params: IReviewGetParams): Promise<Array<IReviewSummary>> {
      return send('review.Get', params)
    },
    /**
     * GetByID returns full review details.
     */
    getByID(params: IReviewGetByIDParams): Promise<IReview> {
      return send('review.GetByID', params)
    },
    /**
     * Issues returns list of issues for a review.
     */
    issues(params: IReviewIssuesParams): Promise<Array<IIssue>> {
      return send('review.Issues', params)
    },
    /**
     * IssuesByProject returns list of issues for a project with cursor-based pagination.
     */
    issuesByProject(params: IReviewIssuesByProjectParams): Promise<Array<IIssue>> {
      return send('review.IssuesByProject', params)
    },
    /**
     * Projects returns list of all projects with review stats.
     */
    projects(): Promise<Array<IProject>> {
      return send('review.Projects')
    },
    /**
     * SetComment updates comment for an issue.
     */
    setComment(params: IReviewSetCommentParams): Promise<boolean> {
      return send('review.SetComment', params)
    }
  }
})
