import HttpRpcClient from './HttpRpcClient'
import { factory } from './factory.generated'

// Re-export types from generated file
export type { IIssueStats as IssueStats, ILastReview as LastReview, IModelInfo as ModelInfo } from './factory.generated'
export type { IProject as Project, IReview as Review, IReviewSummary as ReviewSummary } from './factory.generated'
export type { IReviewFile as ReviewFile, IReviewFileSummary as ReviewFileSummary } from './factory.generated'
export type { IIssue as Issue, IReviewFilters as ReviewFilters, IIssueFilters as IssueFilters } from './factory.generated'

const client = new HttpRpcClient({ url: '/v1/rpc/', isClient: true })

export default factory(client.call)
