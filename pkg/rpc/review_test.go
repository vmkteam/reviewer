package rpc

import (
	"context"
	"testing"
	"time"

	"reviewsrv/pkg/db"
	dbtest "reviewsrv/pkg/db/test"
	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedReview persists a review with an explicit role, externalId and judge/member
// cost; the fusion variant also carries one issue with provenance sources.
func seedReview(t *testing.T, rm *reviewer.ReviewManager, pr *reviewer.Project, role, ext string, cost float64, sources []string) *reviewer.Review {
	rf := reviewer.ReviewFile{ReviewFile: db.ReviewFile{ReviewType: reviewer.ReviewTypeCode, Summary: "s"}}
	if len(sources) > 0 {
		rf.Issues = reviewer.Issues{{Issue: db.Issue{
			Title: "race", Severity: reviewer.SeverityHigh, IssueType: "concurrency",
			File: "a.go", Lines: "1", Sources: db.IssueSources(sources),
		}}}
	}
	rv := &reviewer.Review{
		Review: db.Review{
			Title: role, ExternalID: ext, Author: "tester", CreatedAt: time.Now(),
			ReviewRole: role, ModelInfo: db.ReviewModelInfo{Model: "opus", CostUsd: cost},
		},
		ReviewFiles: reviewer.ReviewFiles{rf},
	}
	created, err := rm.CreateReview(t.Context(), pr, rv)
	require.NoError(t, err)
	return created
}

func cleanupSeed(t *testing.T, dbc db.DB, reviews ...*reviewer.Review) {
	t.Cleanup(func() {
		ctx := context.Background()
		for _, rv := range reviews {
			dbc.ExecContext(ctx, `UPDATE reviews SET "parentReviewId" = NULL WHERE "reviewId" = ?`, rv.ID)
		}
		for _, rv := range reviews {
			for _, f := range rv.ReviewFiles {
				for _, iss := range f.Issues {
					dbc.ModelContext(ctx, &db.Issue{ID: iss.ID}).WherePK().Delete()
				}
				dbc.ModelContext(ctx, &db.ReviewFile{ID: f.ID}).WherePK().Delete()
			}
			dbc.ModelContext(ctx, &db.Review{ID: rv.ID}).WherePK().Delete()
		}
	})
}

func TestDBReviewService_GetByIDFusionBreakdown(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	pr, prCl := dbtest.Project(t, dbc, nil, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(prCl)
	rm := reviewer.NewReviewManager(dbc)
	proj := reviewer.NewProject(pr)
	svc := NewReviewService(dbc)

	m1 := seedReview(t, rm, proj, reviewer.ReviewRoleMember, "MR-7", 2.0, nil)
	m2 := seedReview(t, rm, proj, reviewer.ReviewRoleMember, "MR-7", 3.0, nil)
	fusion := seedReview(t, rm, proj, reviewer.ReviewRoleFusion, "MR-7", 4.0, []string{"gpt-5.5", "opus"})
	cleanupSeed(t, dbc, m1, m2, fusion)
	require.NoError(t, rm.LinkMembers(t.Context(), proj.ID, fusion.ID, []int{m1.ID, m2.ID}))

	t.Run("fusion exposes panel breakdown and summed member cost", func(t *testing.T) {
		got, err := svc.GetByID(t.Context(), fusion.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, reviewer.ReviewRoleFusion, got.ReviewRole)
		require.Len(t, got.Members, 2)
		assert.ElementsMatch(t, []int{m1.ID, m2.ID}, []int{got.Members[0].ID, got.Members[1].ID})
		assert.InDelta(t, 5.0, got.PanelCostUsd, 1e-9, "panel cost sums members, not the judge")
	})

	t.Run("a member reports its parent and no breakdown", func(t *testing.T) {
		got, err := svc.GetByID(t.Context(), m1.ID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, reviewer.ReviewRoleMember, got.ReviewRole)
		require.NotNil(t, got.ParentReviewID)
		assert.Equal(t, fusion.ID, *got.ParentReviewID)
		assert.Empty(t, got.Members)
	})

	t.Run("fusion issues expose provenance sources", func(t *testing.T) {
		issues, err := svc.Issues(t.Context(), fusion.ID, nil)
		require.NoError(t, err)
		require.Len(t, issues, 1)
		assert.Equal(t, []string{"gpt-5.5", "opus"}, issues[0].Sources)
	})
}
