//nolint:staticcheck
package rpc

import (
	"context"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/reviewer"

	"github.com/vmkteam/zenrpc/v2"
)

type ReviewService struct {
	rm *reviewer.ReviewManager
	pm *reviewer.ProjectManager
	zenrpc.Service
}

// NewReviewService creates a JSON-RPC service for review and project operations.
func NewReviewService(dbc db.DB) *ReviewService {
	return &ReviewService{
		rm: reviewer.NewReviewManager(dbc),
		pm: reviewer.NewProjectManager(dbc),
	}
}

func (s ReviewService) checkProject(ctx context.Context, projectID int) error {
	p, err := s.pm.GetByID(ctx, projectID)
	if err != nil {
		return newInternalError(err)
	}
	if p == nil {
		return ErrNotFound
	}
	return nil
}

func (s ReviewService) checkIssue(ctx context.Context, issueID int) error {
	issue, err := s.rm.IssueByID(ctx, issueID)
	if err != nil {
		return newInternalError(err)
	}
	if issue == nil {
		return ErrNotFound
	}
	return nil
}

// Projects returns list of all projects with review stats.
//
//zenrpc:500 Internal Error
func (s ReviewService) Projects(ctx context.Context) ([]Project, error) {
	projects, err := s.pm.List(ctx)
	if err != nil {
		return nil, newInternalError(err)
	}

	stats, err := s.rm.ProjectsStats(ctx)
	if err != nil {
		return nil, newInternalError(err)
	}

	result := newProjects(projects)
	for i := range result {
		if st, ok := stats[result[i].ID]; ok {
			result[i].ReviewCount = st.ReviewCount
			result[i].LastReview = &LastReview{
				CreatedAt:    st.CreatedAt,
				Author:       st.Author,
				TrafficLight: st.TrafficLight,
			}
		}
	}

	return result, nil
}

// ProjectByID returns a single project by ID.
//
//zenrpc:projectId Project ID
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) ProjectByID(ctx context.Context, projectId int) (*Project, error) {
	project, err := s.pm.GetByID(ctx, projectId)
	if err != nil {
		return nil, newInternalError(err)
	}
	if project == nil {
		return nil, ErrNotFound
	}

	return newProject(project), nil
}

// Get returns list of reviews for a project.
//
//zenrpc:projectId Project ID
//zenrpc:filters Review filters
//zenrpc:fromReviewId Cursor for infinite scroll pagination
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) Get(ctx context.Context, projectId int, filters *ReviewFilters, fromReviewId *int) ([]ReviewSummary, error) {
	if err := s.checkProject(ctx, projectId); err != nil {
		return nil, err
	}

	reviews, err := s.rm.ListReviews(ctx, filters.ToDomain(projectId, fromReviewId), 50)
	if err != nil {
		return nil, newInternalError(err)
	}

	if err := s.rm.FillLastVersions(ctx, reviews); err != nil {
		return nil, newInternalError(err)
	}

	return newReviewSummaries(reviews), nil
}

// Count returns count of reviews for a project.
//
//zenrpc:projectId Project ID
//zenrpc:filters Review filters
//zenrpc:return int
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) Count(ctx context.Context, projectId int, filters *ReviewFilters) (int, error) {
	if err := s.checkProject(ctx, projectId); err != nil {
		return 0, err
	}

	count, err := s.rm.CountReviews(ctx, filters.ToDomain(projectId, nil))
	if err != nil {
		return 0, newInternalError(err)
	}

	return count, nil
}

// GetByID returns full review details.
//
//zenrpc:reviewId Review ID
//zenrpc:return Review
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) GetByID(ctx context.Context, reviewId int) (*Review, error) {
	rv, err := s.rm.GetReview(ctx, reviewId)
	if err != nil {
		return nil, newInternalError(err)
	}
	if rv == nil {
		return nil, ErrNotFound
	}

	reviews := reviewer.Reviews{*rv}
	if err := s.rm.FillLastVersions(ctx, reviews); err != nil {
		return nil, newInternalError(err)
	}

	return newReview(&reviews[0]), nil
}

// Issues returns list of issues for a review.
//
//zenrpc:reviewId Review ID
//zenrpc:filters Issue filters
//zenrpc:return []Issue
//zenrpc:500 Internal Error
func (s ReviewService) Issues(ctx context.Context, reviewId int, filters *IssueFilters) ([]Issue, error) {
	issues, err := s.rm.ListIssues(ctx, filters.ToDomain(reviewId), 500)
	if err != nil {
		return nil, newInternalError(err)
	}

	return newIssues(issues), nil
}

// CountIssues returns count of issues for a review.
//
//zenrpc:reviewId Review ID
//zenrpc:filters Issue filters
//zenrpc:return int
//zenrpc:500 Internal Error
func (s ReviewService) CountIssues(ctx context.Context, reviewId int, filters *IssueFilters) (int, error) {
	count, err := s.rm.CountIssues(ctx, filters.ToDomain(reviewId))
	if err != nil {
		return 0, newInternalError(err)
	}

	return count, nil
}

// IssuesByProject returns list of issues for a project with cursor-based pagination.
//
//zenrpc:projectId Project ID
//zenrpc:filters Issue filters (severity, issueType, reviewType, isFalsePositive)
//zenrpc:fromIssueId Cursor for infinite scroll pagination
//zenrpc:return []Issue
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) IssuesByProject(ctx context.Context, projectId int, filters *IssueFilters, fromIssueId *int) ([]Issue, error) {
	if err := s.checkProject(ctx, projectId); err != nil {
		return nil, err
	}

	search := filters.ToDomainByProject(projectId)
	search.FromIssueID = fromIssueId

	issues, err := s.rm.ListIssuesByProject(ctx, search, 50)
	if err != nil {
		return nil, newInternalError(err)
	}

	return newIssues(issues), nil
}

// CountIssuesByProject returns count of issues for a project.
//
//zenrpc:projectId Project ID
//zenrpc:filters Issue filters (severity, issueType, reviewType, isFalsePositive)
//zenrpc:return int
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ReviewService) CountIssuesByProject(ctx context.Context, projectId int, filters *IssueFilters) (int, error) {
	if err := s.checkProject(ctx, projectId); err != nil {
		return 0, err
	}

	count, err := s.rm.CountIssues(ctx, filters.ToDomainByProject(projectId))
	if err != nil {
		return 0, newInternalError(err)
	}

	return count, nil
}

// Feedback updates false positive flag for an issue.
//
//zenrpc:issueId Issue ID
//zenrpc:isFalsePositive False positive flag (true = false positive, false = confirmed, null = unprocessed)
//zenrpc:return bool
//zenrpc:404 Not Found
//zenrpc:500 Internal Error
func (s ReviewService) Feedback(ctx context.Context, issueId int, isFalsePositive *bool) (bool, error) {
	if err := s.checkIssue(ctx, issueId); err != nil {
		return false, err
	}

	ok, err := s.rm.SetFeedback(ctx, issueId, isFalsePositive)
	if err != nil {
		return false, newInternalError(err)
	}

	return ok, nil
}

// SetComment updates comment for an issue.
//
//zenrpc:issueId Issue ID
//zenrpc:comment Developer comment (max 255 chars, null to clear)
//zenrpc:return bool
//zenrpc:400 Bad Request
//zenrpc:404 Not Found
//zenrpc:500 Internal Error
func (s ReviewService) SetComment(ctx context.Context, issueId int, comment *string) (bool, error) {
	if comment != nil && len(*comment) > 255 {
		return false, ErrBadRequest
	}
	if err := s.checkIssue(ctx, issueId); err != nil {
		return false, err
	}

	ok, err := s.rm.SetComment(ctx, issueId, comment)
	if err != nil {
		return false, newInternalError(err)
	}
	return ok, nil
}
