package rest

import (
	"testing"
	"time"

	"reviewsrv/pkg/db"
	dbtest "reviewsrv/pkg/db/test"
	"reviewsrv/pkg/reviewer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A multi-review member uploads with reviewRole=member; a single review leaves it
// empty. Verify the full server-side path (draft → ToModel → CreateReview → DB)
// persists the role, and that an empty role maps to the canonical "single".
func TestDBReviewDraft_ReviewRolePersists(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	pr, prCl := dbtest.Project(t, dbc, nil, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(prCl)
	rm := reviewer.NewReviewManager(dbc)

	cases := []struct {
		name     string
		role     string
		wantRole string
	}{
		{"member role persists", reviewer.ReviewRoleMember, reviewer.ReviewRoleMember},
		{"empty role becomes single", "", reviewer.ReviewRoleSingle},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			draft := ReviewDraft{
				Review: ReviewDraftMeta{Title: "role test", CreatedAt: time.Now(), ReviewRole: tc.role},
				Files:  []ReviewDraftFile{{ReviewType: reviewer.ReviewTypeCode, Summary: "summary"}},
				Issues: []ReviewDraftIssue{{
					LocalID: "C1", Severity: reviewer.SeverityLow, Title: "finding",
					FileType: reviewer.ReviewTypeCode, IssueType: "naming", File: "main.go", Lines: "1",
				}},
			}

			model := draft.ToModel()
			created, err := rm.CreateReview(t.Context(), reviewer.NewProject(pr), &model)
			require.NoError(t, err)
			t.Cleanup(func() { cleanupReview(t, dbc, created) })

			got := db.Review{ID: created.ID}
			require.NoError(t, dbc.ModelContext(t.Context(), &got).WherePK().Select())
			assert.Equal(t, tc.wantRole, got.ReviewRole)
		})
	}
}

// A fusion review persists per-issue provenance (issues.sources) and, via
// LinkMembers, becomes the parent of its panel members (parentReviewId).
func TestDBReviewDraft_FusionSourcesAndLinking(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	pr, prCl := dbtest.Project(t, dbc, nil, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(prCl)
	rm := reviewer.NewReviewManager(dbc)
	proj := reviewer.NewProject(pr)

	memberDraft := ReviewDraft{
		Review: ReviewDraftMeta{Title: "member", CreatedAt: time.Now(), ReviewRole: reviewer.ReviewRoleMember},
		Files:  []ReviewDraftFile{{ReviewType: reviewer.ReviewTypeCode, Summary: "s"}},
	}
	mm := memberDraft.ToModel()
	member, err := rm.CreateReview(t.Context(), proj, &mm)
	require.NoError(t, err)

	fusionDraft := ReviewDraft{
		Review: ReviewDraftMeta{Title: "fusion", CreatedAt: time.Now(), ReviewRole: reviewer.ReviewRoleFusion},
		Files:  []ReviewDraftFile{{ReviewType: reviewer.ReviewTypeCode, Summary: "s"}},
		Issues: []ReviewDraftIssue{{
			LocalID: "C1", Severity: reviewer.SeverityHigh, Title: "race", FileType: reviewer.ReviewTypeCode,
			IssueType: "concurrency", File: "a.go", Lines: "1", Sources: []string{"gpt-5.5", "deepseek-v4-pro"},
		}},
	}
	fm := fusionDraft.ToModel()
	fusion, err := rm.CreateReview(t.Context(), proj, &fm)
	require.NoError(t, err)

	// Delete the member (child) before the fusion (parent) to satisfy the self-fk.
	t.Cleanup(func() {
		cleanupReview(t, dbc, member)
		cleanupReview(t, dbc, fusion)
	})

	// Provenance persisted on the fusion issue.
	gotIssue := db.Issue{ID: fusion.ReviewFiles[0].Issues[0].ID}
	require.NoError(t, dbc.ModelContext(t.Context(), &gotIssue).WherePK().Select())
	assert.Equal(t, db.IssueSources{"gpt-5.5", "deepseek-v4-pro"}, gotIssue.Sources)

	// LinkMembers points the member at the fusion.
	require.NoError(t, rm.LinkMembers(t.Context(), fusion.ID, []int{member.ID}))
	gotMember := db.Review{ID: member.ID}
	require.NoError(t, dbc.ModelContext(t.Context(), &gotMember).WherePK().Select())
	require.NotNil(t, gotMember.ParentReviewID)
	assert.Equal(t, fusion.ID, *gotMember.ParentReviewID)
}
