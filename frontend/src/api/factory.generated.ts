// @ts-nocheck
/* Code generated from jsonrpc schema by rpcgen v2.5.x with typescript v1.0.0; DO NOT EDIT. */
/* eslint-disable */
export interface IIssue {
  issueId: number,
  title: string,
  severity: string,
  description: string,
  content: string,
  file: string,
  lines: string,
  issueType: string,
  reviewType: string,
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
  reviewFiles: Array<IReviewFile>
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

export interface IReviewSetCommentParams {
  issueId: number,
  comment?: string
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

export interface IReviewSummary {
  reviewId: number,
  title: string,
  externalId: string,
  trafficLight: string,
  author: string,
  sourceBranch: string,
  targetBranch: string,
  createdAt: string,
  reviewFiles: Array<IReviewFileSummary>
}

export const factory = (send: any) => ({
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
