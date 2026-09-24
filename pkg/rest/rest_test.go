package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"reviewsrv/pkg/db"
	dbtest "reviewsrv/pkg/db/test"
	"reviewsrv/pkg/reviewer"
	"reviewsrv/pkg/slack"

	"github.com/labstack/echo/v4"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vmkteam/embedlog"
)

// newTestHandler bootstraps the test DB and a Handler wired without a Slack
// notifier — the project-instructions endpoint never touches it.
func newTestHandler(t *testing.T) *Handler {
	t.Helper()
	dbc, _ := dbtest.Setup(t)
	return NewHandler(dbc, nil, "http://localhost", nil)
}

// callInstructionsHandler builds a fresh Echo context with `:id` set to param
// and runs ProjectInstructionsMarkdown against a real-DB Handler.
func callInstructionsHandler(t *testing.T, h *Handler, param string) (*httptest.ResponseRecorder, error) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/v1/rpc/project-instructions-"+param, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetParamNames("id")
	c.SetParamValues(param)
	err := h.ProjectInstructionsMarkdown(c)
	return rec, err
}

func TestDBProjectInstructionsMarkdown_BadRequest(t *testing.T) {
	h := newTestHandler(t)

	cases := []struct {
		name  string
		param string
	}{
		{"no .md suffix", "42"},
		{"non-numeric id", "abc.md"},
		{"zero id", "0.md"},
		{"negative id", "-1.md"},
		{"empty id", ".md"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := callInstructionsHandler(t, h, tc.param)
			var httpErr *echo.HTTPError
			require.ErrorAs(t, err, &httpErr)
			assert.Equal(t, http.StatusBadRequest, httpErr.Code)
		})
	}
}

func TestDBProjectInstructionsMarkdown_NotFound(t *testing.T) {
	h := newTestHandler(t)

	// 999999 is never a real project id in the test DB.
	_, err := callInstructionsHandler(t, h, "999999.md")
	var httpErr *echo.HTTPError
	require.ErrorAs(t, err, &httpErr)
	assert.Equal(t, http.StatusNotFound, httpErr.Code)
}

func TestDBProjectInstructionsMarkdown_OK(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	pr, prCl := dbtest.Project(t, dbc, nil, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(prCl)

	// OK path needs the same dbc as the project, so wire NewHandler inline
	// rather than via newTestHandler.
	h := NewHandler(dbc, nil, "http://localhost", nil)

	rec, err := callInstructionsHandler(t, h, strconv.Itoa(pr.ID)+".md")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "text/markdown; charset=utf-8", rec.Header().Get("Content-Type"))

	body := rec.Body.String()
	assert.Contains(t, body, "# Project review rules from accepted risks")
	// projectKey is a token for /v1/prompt/:projectKey/ — must not leak via
	// the no-auth instructions endpoint (regression guard).
	assert.NotContains(t, body, pr.ProjectKey)

	// Sanity: ErrProjectNotFound is the error the handler maps to 404.
	_ = reviewer.ErrProjectNotFound
}

func TestDBProjectInstructionsMarkdown_RendersIgnoredIssues(t *testing.T) {
	// End-to-end happy path: project has ignored issues → markdown body
	// contains their titles, grouped by reviewType. This is the real-world
	// use case (project accumulated accepted risks); the empty-project
	// variant above only verifies frame/header rendering.
	dbc, _ := dbtest.Setup(t)
	ensureIssueStatuses(t, dbc)

	pr, prCl := dbtest.Project(t, dbc, nil, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(prCl)

	rm := reviewer.NewReviewManager(dbc)
	rv := seedIssuesForProject(t, rm, reviewer.NewProject(pr))
	t.Cleanup(func() { cleanupReview(t, dbc, rv) })

	// Mark first issue as Ignored — it should appear in the markdown.
	ignoredID := rv.ReviewFiles[0].Issues[0].ID
	_, err := rm.SetFeedback(t.Context(), ignoredID, db.StatusIgnored)
	require.NoError(t, err)

	h := NewHandler(dbc, nil, "http://localhost", nil)

	rec, err := callInstructionsHandler(t, h, strconv.Itoa(pr.ID)+".md")
	require.NoError(t, err)
	assert.Equal(t, http.StatusOK, rec.Code)

	body := rec.Body.String()
	assert.Contains(t, body, rv.ReviewFiles[0].Issues[0].Title, "ignored issue title must appear in markdown")
	assert.Contains(t, body, "### "+reviewer.ReviewTypeCode, "issues grouped by reviewType")
}

func ensureIssueStatuses(t *testing.T, dbc db.DB) {
	t.Helper()
	_, err := dbc.ExecContext(t.Context(), `INSERT INTO "statuses" ("statusId", "title", "alias") VALUES (4, 'Valid', 'valid'), (5, 'FalsePositive', 'falsePositive'), (6, 'Ignored', 'ignored') ON CONFLICT DO NOTHING`)
	require.NoError(t, err)
}

func seedIssuesForProject(t *testing.T, rm *reviewer.ReviewManager, pr *reviewer.Project) *reviewer.Review {
	t.Helper()
	rv := &reviewer.Review{
		Review: db.Review{Title: "Test Review", CreatedAt: time.Now()},
		ReviewFiles: reviewer.ReviewFiles{{
			ReviewFile: db.ReviewFile{ReviewType: reviewer.ReviewTypeCode, Content: "code review", Summary: "code summary"},
			Issues: reviewer.Issues{
				{Issue: db.Issue{Title: "Ignored finding A", Severity: reviewer.SeverityLow, IssueType: "naming", Description: "desc", Content: "content", File: "main.go", Lines: "10"}},
			},
		}},
	}
	created, err := rm.CreateReview(t.Context(), pr, rv)
	require.NoError(t, err)
	return created
}

func cleanupReview(t *testing.T, dbc db.DB, rv *reviewer.Review) {
	t.Helper()
	// t.Context() is already canceled by the time t.Cleanup runs, so use a
	// fresh background context for the cleanup queries.
	ctx := context.Background()
	for _, rf := range rv.ReviewFiles {
		for _, iss := range rf.Issues {
			if _, err := dbc.ModelContext(ctx, &db.Issue{ID: iss.ID}).WherePK().Delete(); err != nil {
				t.Logf("cleanup issue %d: %v", iss.ID, err)
			}
		}
		if _, err := dbc.ModelContext(ctx, &db.ReviewFile{ID: rf.ID}).WherePK().Delete(); err != nil {
			t.Logf("cleanup reviewFile %d: %v", rf.ID, err)
		}
	}
	if _, err := dbc.ModelContext(ctx, &db.Review{ID: rv.ID}).WherePK().Delete(); err != nil {
		t.Logf("cleanup review %d: %v", rv.ID, err)
	}
}

func TestDBCreateReviewCountsOkRun(t *testing.T) {
	dbc, _ := dbtest.Setup(t)
	pr, prCl := dbtest.Project(t, dbc, nil, dbtest.WithProjectRelations, dbtest.WithFakeProject)
	t.Cleanup(prCl)
	metrics := reviewer.NewRunMetrics("claude")
	reg := prometheus.NewRegistry()
	metrics.Register(reg)
	h := NewHandler(dbc, nil, "http://localhost", metrics)

	body := `{"review":{"title":"t","commitHash":"abc","sourceBranch":"f","targetBranch":"master","author":"dev",
		"createdAt":"2026-09-24T12:00:00Z","modelInfo":{"model":"opus","costUsd":1.25},"runnerProfile":{"runner":"claude"}},
		"files":[{"reviewType":"code","summary":"ok","isAccepted":true}],"issues":[]}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := echo.New().NewContext(req, rec)
	c.SetParamNames("projectKey")
	c.SetParamValues(pr.ProjectKey)

	require.NoError(t, h.CreateReview(c))
	id, err := strconv.Atoi(rec.Body.String())
	require.NoError(t, err)
	t.Cleanup(func() { // hard delete: the project's cleanup needs the rows gone
		ctx := context.Background()
		_, _ = dbc.ModelContext(ctx, (*db.ReviewFile)(nil)).Where(`"reviewId" = ?`, id).Delete()
		_, _ = dbc.ModelContext(ctx, &db.Review{ID: id}).WherePK().Delete()
	})

	// The profile's runner labels the run; the uploaded review is its ok outcome.
	runs, cost := okRunCounters(t, reg, pr.Title, "claude")
	assert.InDelta(t, 1, runs, 0)
	assert.InDelta(t, 1.25, cost, 1e-9)
}

// okRunCounters reads the ok-run and cost counters of project and runner.
func okRunCounters(t *testing.T, reg *prometheus.Registry, project, runner string) (runs, cost float64) {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	for _, mf := range mfs {
		for _, m := range mf.GetMetric() {
			l := map[string]string{}
			for _, lp := range m.GetLabel() {
				l[lp.GetName()] = lp.GetValue()
			}
			if l["project"] != project || l["runner"] != runner || l["status"] != reviewer.RunStatusOK {
				continue
			}
			switch mf.GetName() {
			case "reviewer_runs_total":
				runs = m.GetCounter().GetValue()
			case "reviewer_run_cost_usd_total":
				cost = m.GetCounter().GetValue()
			}
		}
	}
	return runs, cost
}

func TestNotifySlackSkipsMemberReviews(t *testing.T) {
	var posts atomic.Int32
	webhook := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { posts.Add(1) }))
	t.Cleanup(webhook.Close)
	h := &Handler{notifier: slack.NewNotifier(embedlog.NewDevLogger()), baseURL: "http://localhost"}
	project := &reviewer.Project{Project: db.Project{Title: "p", SlackChannel: &db.SlackChannel{WebhookURL: webhook.URL}}}

	h.notifySlack(project, &reviewer.Review{Review: db.Review{ID: 1, ReviewRole: reviewer.ReviewRoleMember}})
	h.notifySlack(project, &reviewer.Review{Review: db.Review{ID: 2, ReviewRole: reviewer.ReviewRoleSingle}})

	require.Eventually(t, func() bool { return posts.Load() == 1 }, 5*time.Second, 10*time.Millisecond)
	assert.Never(t, func() bool { return posts.Load() > 1 }, 200*time.Millisecond, 10*time.Millisecond, "a member review is not announced")
}
