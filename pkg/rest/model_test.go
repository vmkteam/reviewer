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
