package reviewer

import (
	"context"
	"fmt"
	"strings"

	"reviewsrv/pkg/db"
)

// EnvTrackerToken is the env var reviewctl exports the task-tracker token into
// for runner subprocesses; assembled prompts reference it as $REVIEW_TRACKER_TOKEN
// instead of the real secret. Canonical definition — pkg/reviewer/runner and
// cmd/reviewctl alias it so the prompt reference and the injected env can't drift.
const EnvTrackerToken = "REVIEW_TRACKER_TOKEN"

type ProjectManager struct {
	repo       db.ProjectRepo
	reviewRepo db.ReviewRepo
}

// NewProjectManager creates a new ProjectManager.
func NewProjectManager(dbc db.DB) *ProjectManager {
	return &ProjectManager{
		repo:       db.NewProjectRepo(dbc).WithEnabledOnly(),
		reviewRepo: db.NewReviewRepo(dbc).WithEnabledAndIssueFilters(),
	}
}

// GetByID returns a project by its ID.
func (pm *ProjectManager) GetByID(ctx context.Context, id int) (*Project, error) {
	p, err := pm.repo.ProjectByID(ctx, id, db.WithColumns(db.TableColumns, db.Columns.Project.TaskTracker))

	return NewProject(p), err
}

// GetByKey returns a project by its key with all relations.
func (pm *ProjectManager) GetByKey(ctx context.Context, projectKey string) (*Project, error) {
	p, err := pm.repo.OneProject(ctx, &db.ProjectSearch{ProjectKey: &projectKey}, pm.repo.FullProject())

	return NewProject(p), err
}

// resolvePrimaryProfile returns a project's primary runner profile: its pinned
// profile, or the global default when none is pinned.
func (pm *ProjectManager) resolvePrimaryProfile(ctx context.Context, p *db.Project) (*db.RunnerProfile, error) {
	if p.RunnerProfileID != nil {
		return pm.repo.RunnerProfileByID(ctx, *p.RunnerProfileID)
	}
	isDefault := true
	return pm.repo.OneRunnerProfile(ctx, &db.RunnerProfileSearch{IsDefault: &isDefault})
}

// ReviewSetup is the resolved review run set for a project:
//   - Primary: the project's pinned profile (or the global default) — the primary
//     runner and the promotion target; nil when no default exists (callers treat
//     that as "no runner profile", same as a nil setup);
//   - Panel: the additional members (runnerProfileIds, resolved by id with order
//     and duplicates preserved; ids that no longer resolve — disabled/deleted —
//     are skipped so a stale entry can't abort the run);
//   - Judge: the judge profile (judgeRunnerProfileId), or nil when unset or no
//     longer resolvable → single review (its presence is the multi-review trigger);
//   - Tracker: the project's enabled task tracker (nil when absent or disabled),
//     handed to runners out-of-band so its token never enters the prompt.
type ReviewSetup struct {
	Primary *RunnerProfile
	Panel   []*RunnerProfile
	Judge   *RunnerProfile
	Tracker *TaskTracker
}

// ReviewProfiles resolves the full multi-review setup for a project key in one
// project load. Returns nil when the project key is unknown.
func (pm *ProjectManager) ReviewProfiles(ctx context.Context, projectKey string) (*ReviewSetup, error) {
	p, err := pm.repo.OneProject(ctx, &db.ProjectSearch{ProjectKey: &projectKey}, db.WithColumns(db.TableColumns, db.Columns.Project.TaskTracker))
	if err != nil || p == nil {
		return nil, err
	}

	primary, err := pm.resolvePrimaryProfile(ctx, p)
	if err != nil {
		return nil, err
	}
	setup := &ReviewSetup{Primary: NewRunnerProfile(primary)}

	for _, id := range p.RunnerProfileIDs {
		rp, rerr := pm.repo.RunnerProfileByID(ctx, id)
		if rerr != nil {
			return nil, rerr
		}
		if rp != nil {
			setup.Panel = append(setup.Panel, NewRunnerProfile(rp))
		}
	}

	if p.JudgeRunnerProfileID != nil {
		judge, jerr := pm.repo.RunnerProfileByID(ctx, *p.JudgeRunnerProfileID)
		if jerr != nil {
			return nil, jerr
		}
		setup.Judge = NewRunnerProfile(judge)
	}

	if p.TaskTracker != nil && p.TaskTracker.StatusID == db.StatusEnabled {
		setup.Tracker = NewTaskTracker(p.TaskTracker)
	}

	return setup, nil
}

// List returns all enabled projects.
func (pm *ProjectManager) List(ctx context.Context) (Projects, error) {
	projects, err := pm.repo.ProjectsByFilters(ctx, nil, db.PagerNoLimit, pm.repo.DefaultProjectSort(), db.WithColumns(db.TableColumns, db.Columns.Project.TaskTracker))
	if err != nil {
		return nil, err
	}

	return NewProjects(projects), nil
}

// Prompt returns an assembled prompt for the project. tokenEnv controls the
// tracker section: true substitutes the $REVIEW_TRACKER_TOKEN env reference
// into {{TOKEN}} (the secret travels out-of-band via ReviewConfig), false keeps
// the historical behaviour — the real tracker token in the prompt — for legacy
// reviewctl clients that don't export the env var yet.
func (pm *ProjectManager) Prompt(ctx context.Context, projectKey string, tokenEnv bool) (string, error) {
	p, err := pm.repo.OneProject(ctx, &db.ProjectSearch{ProjectKey: &projectKey}, pm.repo.FullProject())
	if err != nil {
		return "", err
	}

	pr := NewProject(p)
	if pr == nil {
		return "", nil
	}

	return pm.createPrompt(ctx, pr, tokenEnv)
}

// promptData is the data structure for the prompt template.
type promptData struct {
	Common        string
	Instructions  string
	Types         []promptType
	FetchPrompt   string
	AcceptedRisks Issues
}

type promptType struct {
	Num  int
	Text string
}

func (pm *ProjectManager) createPrompt(ctx context.Context, pr *Project, tokenEnv bool) (string, error) {
	prompt := pr.Prompt
	if prompt == nil {
		return "", nil
	}

	data := promptData{
		Common: prompt.Common,
		Types: []promptType{
			{1, prompt.Architecture},
			{2, prompt.Code},
			{3, prompt.Security},
			{4, prompt.Tests},
			{5, prompt.Operability},
		},
	}

	if pr.Instructions != nil {
		data.Instructions = *pr.Instructions
	}

	if pr.TaskTracker != nil && pr.TaskTracker.FetchPrompt != "" {
		fp := pr.TaskTracker.FetchPrompt
		switch {
		case tokenEnv:
			// The secret stays out of the prompt: {{TOKEN}} renders as an env
			// reference that reviewctl exports to CLI runner processes; the direct
			// runner's http_fetch tool injects authorization itself.
			fp = strings.ReplaceAll(fp, "{{TOKEN}}", "$"+EnvTrackerToken)
		case pr.TaskTracker.AuthToken != nil:
			// Legacy reviewctl clients don't export REVIEW_TRACKER_TOKEN yet —
			// keep the historical real-token substitution so their curl
			// instructions keep working until the CI image is updated.
			fp = strings.ReplaceAll(fp, "{{TOKEN}}", *pr.TaskTracker.AuthToken)
		}
		// Trackers are often saved with a trailing slash while templates write
		// "{{URL}}/api/..." — the doubled slash makes YouTrack serve its SPA HTML
		// instead of the API response, so normalise before substitution.
		fp = strings.ReplaceAll(fp, "{{URL}}", strings.TrimSuffix(pr.TaskTracker.URL, "/"))
		data.FetchPrompt = fp
	}

	risks, err := pm.acceptedRisks(ctx, pr.ID)
	if err != nil {
		return "", fmt.Errorf("accepted risks: %w", err)
	}
	data.AcceptedRisks = risks

	var b strings.Builder
	if err := promptTemplate.Execute(&b, data); err != nil {
		return "", fmt.Errorf("execute prompt template: %w", err)
	}

	b.WriteString(promptReviewJSON)

	return b.String(), nil
}

// acceptedRisks returns dismissed issues (false positive + ignored) for the project.
func (pm *ProjectManager) acceptedRisks(ctx context.Context, projectID int) (Issues, error) {
	search := &db.IssueSearch{
		StatusIDs:       []int{db.StatusFalsePositive, db.StatusIgnored},
		ReviewProjectID: &projectID,
	}

	issues, err := pm.reviewRepo.IssuesByFilters(ctx, search, db.PagerNoLimit,
		db.WithColumns(db.TableColumns, db.Columns.Issue.Review),
		db.WithSort(db.SortField{Column: db.Columns.Issue.ID, Direction: db.SortAsc}),
	)

	return NewIssues(issues), err
}
