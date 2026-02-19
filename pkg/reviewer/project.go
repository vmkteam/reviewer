package reviewer

import (
	"context"
	"fmt"
	"strings"

	"reviewsrv/pkg/db"
)

type ProjectManager struct {
	repo       db.ProjectRepo
	reviewRepo db.ReviewRepo
}

// NewProjectManager creates a new ProjectManager.
func NewProjectManager(dbc db.DB) *ProjectManager {
	return &ProjectManager{
		repo:       db.NewProjectRepo(dbc).WithEnabledOnly(),
		reviewRepo: db.NewReviewRepo(dbc).WithEnabledOnly(),
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

// List returns all enabled projects.
func (pm *ProjectManager) List(ctx context.Context) (Projects, error) {
	projects, err := pm.repo.ProjectsByFilters(ctx, nil, db.PagerNoLimit, pm.repo.DefaultProjectSort(), db.WithColumns(db.TableColumns, db.Columns.Project.TaskTracker))
	if err != nil {
		return nil, err
	}

	return NewProjects(projects), nil
}

// Prompt returns an assembled prompt for the project.
func (pm *ProjectManager) Prompt(ctx context.Context, projectKey string) (string, error) {
	p, err := pm.repo.OneProject(ctx, &db.ProjectSearch{ProjectKey: &projectKey}, pm.repo.FullProject())
	if err != nil {
		return "", err
	}

	pr := NewProject(p)
	if pr == nil {
		return "", nil
	}

	return pm.createPrompt(ctx, pr)
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

func (pm *ProjectManager) createPrompt(ctx context.Context, pr *Project) (string, error) {
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
		},
	}

	if pr.Instructions != nil {
		data.Instructions = *pr.Instructions
	}

	if pr.TaskTracker != nil && pr.TaskTracker.FetchPrompt != "" {
		fp := pr.TaskTracker.FetchPrompt
		if pr.TaskTracker.AuthToken != nil {
			fp = strings.ReplaceAll(fp, "{{TOKEN}}", *pr.TaskTracker.AuthToken)
		}
		fp = strings.ReplaceAll(fp, "{{URL}}", pr.TaskTracker.URL)
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

// acceptedRisks returns false positive issues for the project.
func (pm *ProjectManager) acceptedRisks(ctx context.Context, projectID int) (Issues, error) {
	isFP := true
	search := &db.IssueSearch{
		IsFalsePositive: &isFP,
		ReviewProjectID: &projectID,
	}

	issues, err := pm.reviewRepo.IssuesByFilters(ctx, search, db.PagerNoLimit,
		db.WithColumns(db.TableColumns, db.Columns.Issue.Review),
		db.WithSort(db.SortField{Column: db.Columns.Issue.ID, Direction: db.SortAsc}),
	)

	return NewIssues(issues), err
}
