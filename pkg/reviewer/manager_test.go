package reviewer

import (
	"context"
	"testing"
	"time"

	"reviewsrv/pkg/db"
	"reviewsrv/pkg/db/test"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestReviewManager(t *testing.T) (*ReviewManager, db.DB) {
	dbc, _ := test.Setup(t)
	return NewReviewManager(dbc), dbc
}

func createTestProject(t *testing.T, dbc db.DB) (*Project, test.Cleaner) {
	pr, cl := test.Project(t, dbc, nil, test.WithProjectRelations, test.WithFakeProject)
	return NewProject(pr), cl
}

func cleanupReview(t *testing.T, dbc db.DB, rv *Review) {
	t.Cleanup(func() {
		ctx := context.Background()
		for _, rf := range rv.ReviewFiles {
			for _, iss := range rf.Issues {
				dbc.ModelContext(ctx, &db.Issue{ID: iss.ID}).WherePK().Delete()
			}
			dbc.ModelContext(ctx, &db.ReviewFile{ID: rf.ID}).WherePK().Delete()
		}
		dbc.ModelContext(ctx, &db.Review{ID: rv.ID}).WherePK().Delete()
	})
}

func createTestReview(t *testing.T, rm *ReviewManager, pr *Project) *Review {
	rv := &Review{
		Review: db.Review{
			Title:        "Test Review",
			Description:  "Test Description",
			ExternalID:   "MR-123",
			CommitHash:   "abc123",
			SourceBranch: "feature/test",
			TargetBranch: "main",
			Author:       "tester",
			CreatedAt:    time.Now(),
			DurationMS:   1000,
		},
		ReviewFiles: ReviewFiles{
			{
				ReviewFile: db.ReviewFile{ReviewType: ReviewTypeCode, Content: "code review content", Summary: "code summary"},
				Issues: Issues{
					{db.Issue{Title: "Missing error check", Severity: SeverityHigh, IssueType: "error-handling", Description: "desc", Content: "content", File: "main.go", Lines: "10-15"}},
					{db.Issue{Title: "Unused var", Severity: SeverityLow, IssueType: "naming", Description: "desc2", Content: "content2", File: "main.go", Lines: "20"}},
				},
			},
			{
				ReviewFile: db.ReviewFile{ReviewType: ReviewTypeSecurity, Content: "security review content", Summary: "security summary"},
				Issues: Issues{
					{db.Issue{Title: "SQL injection", Severity: SeverityCritical, IssueType: "security", Description: "desc3", Content: "content3", File: "db.go", Lines: "50-55"}},
				},
			},
		},
	}

	created, err := rm.CreateReview(t.Context(), pr, rv)
	require.NoError(t, err)
	return created
}

func TestDBReviewManager_CreateReview(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	t.Run("create with files and issues", func(t *testing.T) {
		rv := createTestReview(t, rm, pr)
		cleanupReview(t, dbc, rv)

		assert.NotZero(t, rv.ID)
		assert.Equal(t, pr.ID, rv.ProjectID)
		assert.Equal(t, pr.PromptID, rv.PromptID)
		assert.Equal(t, db.StatusEnabled, rv.StatusID)
		assert.Equal(t, "red", rv.TrafficLight) // 1 critical

		assert.Len(t, rv.ReviewFiles, 2)
		for _, rf := range rv.ReviewFiles {
			assert.NotZero(t, rf.ID)
			assert.Equal(t, rv.ID, rf.ReviewID)
			assert.Equal(t, db.StatusEnabled, rf.StatusID)
			for _, iss := range rf.Issues {
				assert.NotZero(t, iss.ID)
				assert.Equal(t, rv.ID, iss.ReviewID)
				assert.Equal(t, rf.ID, iss.ReviewFileID)
				assert.Equal(t, db.StatusEnabled, iss.StatusID)
			}
		}

		// code file stats
		codeFile := rv.ReviewFiles[0]
		assert.Equal(t, db.ReviewFileIssueStats{High: 1, Low: 1, Total: 2}, codeFile.IssueStats)
		assert.Equal(t, "yellow", codeFile.TrafficLight)

		// security file stats
		secFile := rv.ReviewFiles[1]
		assert.Equal(t, db.ReviewFileIssueStats{Critical: 1, Total: 1}, secFile.IssueStats)
		assert.Equal(t, "red", secFile.TrafficLight)
	})

	t.Run("duplicate reviewType error", func(t *testing.T) {
		rv := &Review{
			ReviewFiles: ReviewFiles{
				{ReviewFile: db.ReviewFile{ReviewType: ReviewTypeCode}},
				{ReviewFile: db.ReviewFile{ReviewType: ReviewTypeCode}},
			},
		}

		_, err := rm.CreateReview(t.Context(), pr, rv)
		assert.ErrorIs(t, err, ErrDuplicateReviewType)
	})
}

func TestDBReviewManager_GetReview(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	t.Run("found with files and issues", func(t *testing.T) {
		got, err := rm.GetReview(t.Context(), rv.ID)
		require.NoError(t, err)
		require.NotNil(t, got)

		assert.Equal(t, rv.ID, got.ID)
		assert.Equal(t, rv.Title, got.Title)
		assert.Len(t, got.ReviewFiles, 2)

		totalIssues := 0
		for _, rf := range got.ReviewFiles {
			totalIssues += len(rf.Issues)
		}
		assert.Equal(t, 3, totalIssues)
	})

	t.Run("not found", func(t *testing.T) {
		got, err := rm.GetReview(t.Context(), -1)
		require.NoError(t, err)
		assert.Nil(t, got)
	})
}

func TestDBReviewManager_ListReviews(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv1 := createTestReview(t, rm, pr)
	rv2 := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv1)
	cleanupReview(t, dbc, rv2)

	t.Run("by projectID", func(t *testing.T) {
		reviews, err := rm.ListReviews(t.Context(), &ReviewSearch{ProjectID: pr.ID}, 100)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(reviews), 2)

		for _, r := range reviews {
			assert.Equal(t, pr.ID, r.ProjectID)
			assert.NotNil(t, r.ReviewFiles)
		}
	})

	t.Run("cursor pagination", func(t *testing.T) {
		reviews, err := rm.ListReviews(t.Context(), &ReviewSearch{ProjectID: pr.ID}, 1)
		require.NoError(t, err)
		assert.Len(t, reviews, 1)

		// next page
		reviews2, err := rm.ListReviews(t.Context(), &ReviewSearch{ProjectID: pr.ID, FromReviewID: &reviews[0].ID}, 1)
		require.NoError(t, err)
		assert.Len(t, reviews2, 1)
		assert.Less(t, reviews2[0].ID, reviews[0].ID)
	})

	t.Run("filter by author", func(t *testing.T) {
		author := "tester"
		reviews, err := rm.ListReviews(t.Context(), &ReviewSearch{ProjectID: pr.ID, Author: &author}, 100)
		require.NoError(t, err)
		for _, r := range reviews {
			assert.Equal(t, "tester", r.Author)
		}
	})
}

func TestDBReviewManager_CountReviews(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	count, err := rm.CountReviews(t.Context(), &ReviewSearch{ProjectID: pr.ID})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 1)
}

func TestDBReviewManager_ReviewFileByKey(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	t.Run("found", func(t *testing.T) {
		rf, err := rm.ReviewFileByKey(t.Context(), rv.ID, ReviewTypeCode, pr.ID)
		require.NoError(t, err)
		require.NotNil(t, rf)
		assert.Equal(t, ReviewTypeCode, rf.ReviewType)
		assert.Equal(t, rv.ID, rf.ReviewID)
	})

	t.Run("invalid review type", func(t *testing.T) {
		_, err := rm.ReviewFileByKey(t.Context(), rv.ID, "invalid", pr.ID)
		assert.ErrorIs(t, err, ErrInvalidReviewType)
	})

	t.Run("wrong project", func(t *testing.T) {
		rf, err := rm.ReviewFileByKey(t.Context(), rv.ID, ReviewTypeCode, -1)
		require.NoError(t, err)
		assert.Nil(t, rf)
	})
}

func TestDBReviewManager_UpdateReviewFileContent(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	rf, err := rm.ReviewFileByKey(t.Context(), rv.ID, ReviewTypeCode, pr.ID)
	require.NoError(t, err)
	require.NotNil(t, rf)

	newContent := "updated content here"
	ok, err := rm.UpdateReviewFileContent(t.Context(), rf, newContent)
	require.NoError(t, err)
	assert.True(t, ok)

	// verify
	rf2, err := rm.ReviewFileByKey(t.Context(), rv.ID, ReviewTypeCode, pr.ID)
	require.NoError(t, err)
	assert.Equal(t, newContent, rf2.Content)
}

func TestDBReviewManager_ProjectsStats(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	stats, err := rm.ProjectsStats(t.Context())
	require.NoError(t, err)
	require.Contains(t, stats, pr.ID)

	ps := stats[pr.ID]
	assert.Equal(t, pr.ID, ps.ProjectID)
	assert.GreaterOrEqual(t, ps.ReviewCount, 1)
	assert.Equal(t, "tester", ps.Author)
	assert.NotEmpty(t, ps.TrafficLight)
}

func TestDBReviewManager_SetFeedback(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	issueID := rv.ReviewFiles[0].Issues[0].ID

	t.Run("set true", func(t *testing.T) {
		isFP := true
		ok, err := rm.SetFeedback(t.Context(), issueID, &isFP)
		require.NoError(t, err)
		assert.True(t, ok)

		iss, err := rm.IssueByID(t.Context(), issueID)
		require.NoError(t, err)
		require.NotNil(t, iss.IsFalsePositive)
		assert.True(t, *iss.IsFalsePositive)
		assert.NotNil(t, iss.ProcessedAt)
	})

	t.Run("set false", func(t *testing.T) {
		isFP := false
		ok, err := rm.SetFeedback(t.Context(), issueID, &isFP)
		require.NoError(t, err)
		assert.True(t, ok)

		iss, err := rm.IssueByID(t.Context(), issueID)
		require.NoError(t, err)
		require.NotNil(t, iss.IsFalsePositive)
		assert.False(t, *iss.IsFalsePositive)
		assert.NotNil(t, iss.ProcessedAt)
	})

	t.Run("reset to nil", func(t *testing.T) {
		ok, err := rm.SetFeedback(t.Context(), issueID, nil)
		require.NoError(t, err)
		assert.True(t, ok)

		iss, err := rm.IssueByID(t.Context(), issueID)
		require.NoError(t, err)
		assert.Nil(t, iss.IsFalsePositive)
		assert.Nil(t, iss.ProcessedAt)
	})
}

func TestDBReviewManager_ListIssues(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	issues, err := rm.ListIssues(t.Context(), &IssueSearch{ReviewID: rv.ID}, 100)
	require.NoError(t, err)
	assert.Len(t, issues, 3)
}

func TestDBReviewManager_CountIssues(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	count, err := rm.CountIssues(t.Context(), &IssueSearch{ReviewID: rv.ID})
	require.NoError(t, err)
	assert.Equal(t, 3, count)
}

func TestDBReviewManager_ListIssuesByProject(t *testing.T) {
	rm, dbc := newTestReviewManager(t)
	pr, prCl := createTestProject(t, dbc)
	t.Cleanup(prCl)

	rv := createTestReview(t, rm, pr)
	cleanupReview(t, dbc, rv)

	issues, err := rm.ListIssuesByProject(t.Context(), &IssueSearch{ProjectID: &pr.ID}, 100)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(issues), 3)

	// verify sorted by ID ASC
	for i := 1; i < len(issues); i++ {
		assert.Less(t, issues[i-1].ID, issues[i].ID)
	}
}
