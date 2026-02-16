import { send } from './client'
import { factory } from './factory.generated'

// Re-export types from generated file
export type { IIssueStats as IssueStats, ILastReview as LastReview, IModelInfo as ModelInfo } from './factory.generated'
export type { IProject as Project, IReview as Review, IReviewSummary as ReviewSummary } from './factory.generated'
export type { IReviewFile as ReviewFile, IReviewFileSummary as ReviewFileSummary } from './factory.generated'
export type { IIssue as Issue, IReviewFilters as ReviewFilters, IIssueFilters as IssueFilters } from './factory.generated'

const generated = factory(send)

const api = {
  review: {
    projects: () => generated.review.projects(),

    get: (projectId: number, filters?: any, fromReviewId?: number) =>
      generated.review.get({ projectId, filters, fromReviewId }),

    count: (projectId: number, filters?: any) =>
      generated.review.count({ projectId, filters }),

    getByID: (reviewId: number) =>
      generated.review.getByID({ reviewId }),

    issues: (reviewId: number, filters?: any) =>
      generated.review.issues({ reviewId, filters }),

    countIssues: (reviewId: number, filters?: any) =>
      generated.review.countIssues({ reviewId, filters }),

    issuesByProject: (projectId: number, filters?: any, fromIssueId?: number) =>
      generated.review.issuesByProject({ projectId, filters, fromIssueId }),

    countIssuesByProject: (projectId: number, filters?: any) =>
      generated.review.countIssuesByProject({ projectId, filters }),

    feedback: (issueId: number, isFalsePositive: boolean | null) =>
      generated.review.feedback({ issueId, isFalsePositive: isFalsePositive ?? undefined }),

    setComment: (issueId: number, comment: string | null) =>
      generated.review.setComment({ issueId, comment: comment ?? undefined }),
  },
}

export default api
