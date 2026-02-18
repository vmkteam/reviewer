package reviewer

import (
	"context"
	"fmt"
	"time"

	"reviewsrv/pkg/db"

	"github.com/go-pg/pg/v10"
)

type ReviewManager struct {
	db.TxManager
	repo db.ReviewRepo
}

// NewReviewManager creates a new ReviewManager.
func NewReviewManager(dbc db.DB) *ReviewManager {
	return &ReviewManager{
		TxManager: db.NewTxManager(&dbc),
		repo:      db.NewReviewRepo(dbc).WithEnabledOnly(),
	}
}

func (rm *ReviewManager) runInLock(ctx context.Context, lockName string, fn func(rm *ReviewManager) error) error {
	return rm.DB().RunInLock(ctx, lockName, func(tx *pg.Tx) error {
		txRM := &ReviewManager{
			TxManager: db.NewTxManager(rm.DB()),
			repo:      rm.repo.WithTransaction(tx),
		}
		txRM.SetTx(tx)

		return fn(txRM)
	})
}

// ReviewFileByKey returns a review file by reviewID and reviewType within a project.
func (rm *ReviewManager) ReviewFileByKey(ctx context.Context, reviewID int, reviewType string, projectID int) (*ReviewFile, error) {
	if !IsValidReviewType(reviewType) {
		return nil, ErrInvalidReviewType
	}

	rv, err := rm.repo.OneReview(ctx, &db.ReviewSearch{ID: &reviewID, ProjectID: &projectID})
	if err != nil || rv == nil {
		return nil, err
	}

	rf, err := rm.repo.OneReviewFile(ctx, &db.ReviewFileSearch{
		ReviewID:   &reviewID,
		ReviewType: &reviewType,
	})

	return NewReviewFile(rf), err
}

// UpdateReviewFileContent updates the content of a review file.
func (rm *ReviewManager) UpdateReviewFileContent(ctx context.Context, rf *ReviewFile, content string) (bool, error) {
	rf.Content = content

	return rm.repo.UpdateReviewFile(ctx, &rf.ReviewFile, db.WithColumns(db.Columns.ReviewFile.Content))
}

func prepareReview(pr *Project, rv *Review) error {
	rv.ProjectID = pr.ID
	rv.PromptID = pr.PromptID
	rv.StatusID = db.StatusEnabled

	seen := make(map[string]struct{}, len(rv.ReviewFiles))
	var totalStats IssueStats
	for i := range rv.ReviewFiles {
		rt := rv.ReviewFiles[i].ReviewType
		if _, ok := seen[rt]; ok {
			return fmt.Errorf("%w: %s", ErrDuplicateReviewType, rt)
		}
		seen[rt] = struct{}{}

		rv.ReviewFiles[i].StatusID = db.StatusEnabled

		stats := calcIssueStats(rv.ReviewFiles[i].Issues)
		rv.ReviewFiles[i].IssueStats = db.ReviewFileIssueStats(stats)
		rv.ReviewFiles[i].TrafficLight = calcTrafficLight(stats)
		totalStats.Add(stats)

		for j := range rv.ReviewFiles[i].Issues {
			rv.ReviewFiles[i].Issues[j].StatusID = db.StatusEnabled
		}
	}
	rv.TrafficLight = calcTrafficLight(totalStats)

	return nil
}

// CreateReview prepares and saves a review with all files and issues in a transaction.
func (rm *ReviewManager) CreateReview(ctx context.Context, pr *Project, rv *Review) (*Review, error) {
	if err := prepareReview(pr, rv); err != nil {
		return nil, err
	}

	err := rm.runInLock(ctx, pr.ProjectKey, func(txRM *ReviewManager) error {
		if _, err := txRM.repo.AddReview(ctx, &rv.Review); err != nil {
			return fmt.Errorf("add review: %w", err)
		}

		for i := range rv.ReviewFiles {
			rv.ReviewFiles[i].ReviewID = rv.ID

			if _, err := txRM.repo.AddReviewFile(ctx, &rv.ReviewFiles[i].ReviewFile); err != nil {
				return fmt.Errorf("add review file: %w", err)
			}

			for j := range rv.ReviewFiles[i].Issues {
				rv.ReviewFiles[i].Issues[j].ReviewID = rv.ID
				rv.ReviewFiles[i].Issues[j].ReviewFileID = rv.ReviewFiles[i].ID

				if _, err := txRM.repo.AddIssue(ctx, &rv.ReviewFiles[i].Issues[j].Issue); err != nil {
					return fmt.Errorf("add issue: %w", err)
				}
			}
		}

		return nil
	})

	return rv, err
}

type lastVersionResult struct {
	ReviewID            int `pg:"reviewId"`
	LastVersionReviewID int `pg:"lastVersionReviewId"`
}

// FillLastVersions fills LastVersionReviewID for reviews that have a newer version
// with the same (projectId, externalId). Only reviews with non-empty externalId are checked.
func (rm *ReviewManager) FillLastVersions(ctx context.Context, reviews Reviews) error {
	if len(reviews) == 0 {
		return nil
	}

	var results []lastVersionResult
	_, err := rm.Conn().QueryContext(ctx, &results, `
		WITH latest AS (
			SELECT DISTINCT ON ("projectId", "externalId")
				"projectId", "externalId", "reviewId" as "lastVersionReviewId"
			FROM reviews
			WHERE "statusId" = ?
			AND "externalId" != ''
			ORDER BY "projectId", "externalId", "reviewId" DESC
		)
		SELECT r."reviewId", l."lastVersionReviewId"
		FROM reviews r
		JOIN latest l ON l."projectId" = r."projectId" AND l."externalId" = r."externalId"
		WHERE r."reviewId" IN (?)
		AND r."externalId" != ''
		AND l."lastVersionReviewId" != r."reviewId"
	`, db.StatusEnabled, pg.In(reviews.IDs()))
	if err != nil {
		return err
	}

	if len(results) == 0 {
		return nil
	}

	idx := make(map[int]int, len(reviews))
	for i := range reviews {
		idx[reviews[i].ID] = i
	}
	for _, r := range results {
		if i, ok := idx[r.ReviewID]; ok {
			id := r.LastVersionReviewID
			reviews[i].LastVersionReviewID = &id
		}
	}
	return nil
}

// ProjectsStats returns review count and last review info for all active projects in one query.
func (rm *ReviewManager) ProjectsStats(ctx context.Context) (map[int]ProjectStats, error) {
	var stats []ProjectStats
	_, err := rm.Conn().QueryContext(ctx, &stats, `
		SELECT DISTINCT ON ("projectId")
			"projectId",
			count(*) OVER (PARTITION BY "projectId") as "reviewCount",
			"createdAt",
			"author",
			"trafficLight"
		FROM reviews
		WHERE "statusId" = ?
		ORDER BY "projectId", "createdAt" DESC
	`, db.StatusEnabled)
	if err != nil {
		return nil, err
	}

	result := make(map[int]ProjectStats, len(stats))
	for _, s := range stats {
		result[s.ProjectID] = s
	}
	return result, nil
}

// ListReviews returns reviews with review files for the given search.
func (rm *ReviewManager) ListReviews(ctx context.Context, search *ReviewSearch, count int) (Reviews, error) {
	dbReviews, err := rm.repo.ReviewsByFilters(ctx, search.ToDB(), db.NewPager(0, count), rm.repo.DefaultReviewSort())
	if err != nil {
		return nil, err
	}

	reviews := NewReviews(dbReviews)
	if len(reviews) == 0 {
		return reviews, nil
	}

	dbRFs, err := rm.repo.ReviewFilesByFilters(ctx, &db.ReviewFileSearch{ReviewIDs: reviews.IDs()}, db.PagerNoLimit)
	if err != nil {
		return nil, err
	}

	rfByReview := NewReviewFiles(dbRFs).GroupByReviewID()
	for i := range reviews {
		reviews[i].ReviewFiles = rfByReview[reviews[i].ID]
	}

	return reviews, nil
}

// CountReviews returns count of reviews matching search.
func (rm *ReviewManager) CountReviews(ctx context.Context, search *ReviewSearch) (int, error) {
	return rm.repo.CountReviews(ctx, search.ToDB())
}

// GetReview returns a review by ID with review files and issues.
func (rm *ReviewManager) GetReview(ctx context.Context, reviewID int) (*Review, error) {
	dbReview, err := rm.repo.ReviewByID(ctx, reviewID)
	if err != nil || dbReview == nil {
		return nil, err
	}

	rv := NewReview(dbReview)

	dbRFs, err := rm.repo.ReviewFilesByFilters(ctx, &db.ReviewFileSearch{ReviewID: &reviewID}, db.PagerNoLimit)
	if err != nil {
		return nil, err
	}
	rv.ReviewFiles = NewReviewFiles(dbRFs)

	dbIssues, err := rm.repo.IssuesByFilters(ctx, &db.IssueSearch{ReviewID: &reviewID}, db.PagerNoLimit, rm.repo.FullIssue())
	if err != nil {
		return nil, err
	}

	issuesByRF := NewIssues(dbIssues).GroupByReviewFileID()
	for i := range rv.ReviewFiles {
		rv.ReviewFiles[i].Issues = issuesByRF[rv.ReviewFiles[i].ID]
	}

	return rv, nil
}

// ListIssues returns issues for a review matching search.
func (rm *ReviewManager) ListIssues(ctx context.Context, search *IssueSearch, count int) (Issues, error) {
	dbIssues, err := rm.repo.IssuesByFilters(ctx, search.ToDB(), db.NewPager(0, count), rm.repo.FullIssue())
	if err != nil {
		return nil, err
	}

	return NewIssues(dbIssues), nil
}

// CountIssues returns count of issues matching search.
func (rm *ReviewManager) CountIssues(ctx context.Context, search *IssueSearch) (int, error) {
	return rm.repo.CountIssues(ctx, search.ToDB(), rm.repo.FullIssue())
}

// ListIssuesByProject returns issues for a project matching search, sorted by issueId ASC.
func (rm *ReviewManager) ListIssuesByProject(ctx context.Context, search *IssueSearch, count int) (Issues, error) {
	dbIssues, err := rm.repo.IssuesByFilters(ctx, search.ToDB(), db.NewPager(0, count),
		rm.repo.FullIssue(),
		db.WithSort(db.SortField{Column: db.Columns.Issue.ID, Direction: db.SortAsc}),
	)
	if err != nil {
		return nil, err
	}

	return NewIssues(dbIssues), nil
}

// IssueByID returns an issue by ID.
func (rm *ReviewManager) IssueByID(ctx context.Context, issueID int) (*Issue, error) {
	issue, err := rm.repo.IssueByID(ctx, issueID)
	if err != nil {
		return nil, err
	}
	if issue == nil {
		return nil, nil
	}
	return NewIssue(issue), nil
}

// SetComment updates comment for an issue.
func (rm *ReviewManager) SetComment(ctx context.Context, issueID int, comment *string) (bool, error) {
	issue := &db.Issue{ID: issueID, Comment: comment}
	return rm.repo.UpdateIssue(ctx, issue, db.WithColumns(db.Columns.Issue.Comment))
}

// SetFeedback updates the isFalsePositive flag on an issue and sets processedAt accordingly.
func (rm *ReviewManager) SetFeedback(ctx context.Context, issueID int, isFalsePositive *bool) (bool, error) {
	issue := &db.Issue{ID: issueID, IsFalsePositive: isFalsePositive}
	if isFalsePositive != nil {
		now := time.Now()
		issue.ProcessedAt = &now
	}

	return rm.repo.UpdateIssue(ctx, issue, db.WithColumns(db.Columns.Issue.IsFalsePositive, db.Columns.Issue.ProcessedAt))
}
