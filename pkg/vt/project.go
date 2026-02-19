package vt

import (
	"bytes"
	"context"
	_ "embed"
	"text/template"

	"reviewsrv/pkg/db"

	"github.com/vmkteam/embedlog"
	"github.com/vmkteam/zenrpc/v2"
)

//go:embed gitlab-ci.yml.tmpl
var gitlabCITmpl string

type ProjectService struct {
	zenrpc.Service
	embedlog.Logger
	projectRepo db.ProjectRepo
	baseURL     string
}

func NewProjectService(dbo db.DB, logger embedlog.Logger, baseURL string) *ProjectService {
	return &ProjectService{
		Logger:      logger,
		projectRepo: db.NewProjectRepo(dbo),
		baseURL:     baseURL,
	}
}

func (s ProjectService) dbSort(ops *ViewOps) db.OpFunc {
	v := s.projectRepo.DefaultProjectSort()
	if ops == nil {
		return v
	}

	switch ops.SortColumn {
	case db.Columns.Project.ID, db.Columns.Project.Title, db.Columns.Project.VcsURL, db.Columns.Project.Language, db.Columns.Project.ProjectKey, db.Columns.Project.PromptID, db.Columns.Project.TaskTrackerID, db.Columns.Project.SlackChannelID, db.Columns.Project.StatusID:
		v = db.WithSort(db.NewSortField(ops.SortColumn, ops.SortDesc))
	}

	return v
}

// Count returns count Projects according to conditions in search params.
//
//zenrpc:search ProjectSearch
//zenrpc:return int
//zenrpc:500 Internal Error
func (s ProjectService) Count(ctx context.Context, search *ProjectSearch) (int, error) {
	count, err := s.projectRepo.CountProjects(ctx, search.ToDB())
	if err != nil {
		return 0, InternalError(err)
	}
	return count, nil
}

// Get returns а list of Projects according to conditions in search params.
//
//zenrpc:search ProjectSearch
//zenrpc:viewOps ViewOps
//zenrpc:return []ProjectSummary
//zenrpc:500 Internal Error
func (s ProjectService) Get(ctx context.Context, search *ProjectSearch, viewOps *ViewOps) ([]ProjectSummary, error) {
	list, err := s.projectRepo.ProjectsByFilters(ctx, search.ToDB(), viewOps.Pager(), s.dbSort(viewOps), s.projectRepo.FullProject())
	if err != nil {
		return nil, InternalError(err)
	}
	projects := make([]ProjectSummary, 0, len(list))
	for i := range list {
		if project := NewProjectSummary(&list[i]); project != nil {
			projects = append(projects, *project)
		}
	}
	return projects, nil
}

// GetByID returns a Project by its ID.
//
//zenrpc:id int
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s ProjectService) GetByID(ctx context.Context, id int) (*Project, error) {
	db, err := s.byID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewProject(db), nil
}

func (s ProjectService) byID(ctx context.Context, id int) (*db.Project, error) {
	db, err := s.projectRepo.ProjectByID(ctx, id, s.projectRepo.FullProject())
	if err != nil {
		return nil, InternalError(err)
	} else if db == nil {
		return nil, ErrNotFound
	}
	return db, nil
}

// Add adds a Project from the query.
//
//zenrpc:project Project
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
func (s ProjectService) Add(ctx context.Context, project Project) (*Project, error) {
	if ve := s.isValid(ctx, project, false); ve.HasErrors() {
		return nil, ve.Error()
	}

	project.ProjectKey = generateUUID()
	db, err := s.projectRepo.AddProject(ctx, project.ToDB())
	if err != nil {
		return nil, InternalError(err)
	}
	return NewProject(db), nil
}

// Update updates the Project data identified by id from the query.
//
//zenrpc:projects Project
//zenrpc:return Project
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s ProjectService) Update(ctx context.Context, project Project) (bool, error) {
	if _, err := s.byID(ctx, project.ID); err != nil {
		return false, err
	}

	if ve := s.isValid(ctx, project, true); ve.HasErrors() {
		return false, ve.Error()
	}

	ok, err := s.projectRepo.UpdateProject(ctx, project.ToDB())
	if err != nil {
		return false, InternalError(err)
	}
	return ok, nil
}

// Delete deletes the Project by its ID.
//
//zenrpc:id int
//zenrpc:return isDeleted
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s ProjectService) Delete(ctx context.Context, id int) (bool, error) {
	if _, err := s.byID(ctx, id); err != nil {
		return false, err
	}

	ok, err := s.projectRepo.DeleteProject(ctx, id)
	if err != nil {
		return false, InternalError(err)
	}
	return ok, err
}

// GitlabCI returns a generated GitLab CI YAML fragment.
//
//zenrpc:targetBranch string
//zenrpc:return string
//zenrpc:500 Internal Error
func (s ProjectService) GitlabCI(_ context.Context, targetBranch string) (string, error) {
	if targetBranch == "" {
		targetBranch = "devel"
	}

	tmpl, err := template.New("gitlab-ci").Parse(gitlabCITmpl)
	if err != nil {
		return "", InternalError(err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, map[string]string{
		"BaseURL":      s.baseURL,
		"TargetBranch": targetBranch,
	}); err != nil {
		return "", InternalError(err)
	}

	return buf.String(), nil
}

// Validate verifies that Project data is valid.
//
//zenrpc:project Project
//zenrpc:return []FieldError
//zenrpc:500 Internal Error
func (s ProjectService) Validate(ctx context.Context, project Project) ([]FieldError, error) {
	isUpdate := project.ID != 0
	if isUpdate {
		_, err := s.byID(ctx, project.ID)
		if err != nil {
			return nil, err
		}
	}

	ve := s.isValid(ctx, project, isUpdate)
	if ve.HasInternalError() {
		return nil, ve.Error()
	}

	return ve.Fields(), nil
}

func (s ProjectService) isValid(ctx context.Context, project Project, isUpdate bool) Validator {
	_ = isUpdate

	var v Validator

	if v.CheckBasic(ctx, project); v.HasInternalError() {
		return v
	}

	// check fks
	if project.PromptID != 0 {
		item, err := s.projectRepo.PromptByID(ctx, project.PromptID)
		if err != nil {
			v.SetInternalError(err)
		} else if item == nil {
			v.Append("promptId", FieldErrorIncorrect)
		}
	}

	if project.TaskTrackerID != nil {
		item, err := s.projectRepo.TaskTrackerByID(ctx, *project.TaskTrackerID)
		if err != nil {
			v.SetInternalError(err)
		} else if item == nil {
			v.Append("taskTrackerId", FieldErrorIncorrect)
		}
	}

	if project.SlackChannelID != nil {
		item, err := s.projectRepo.SlackChannelByID(ctx, *project.SlackChannelID)
		if err != nil {
			v.SetInternalError(err)
		} else if item == nil {
			v.Append("slackChannelId", FieldErrorIncorrect)
		}
	}

	// custom validation starts here
	return v
}

type PromptService struct {
	zenrpc.Service
	embedlog.Logger
	projectRepo db.ProjectRepo
}

func NewPromptService(dbo db.DB, logger embedlog.Logger) *PromptService {
	return &PromptService{
		Logger:      logger,
		projectRepo: db.NewProjectRepo(dbo),
	}
}

func (s PromptService) dbSort(ops *ViewOps) db.OpFunc {
	v := s.projectRepo.DefaultPromptSort()
	if ops == nil {
		return v
	}

	switch ops.SortColumn {
	case db.Columns.Prompt.ID, db.Columns.Prompt.Title, db.Columns.Prompt.Common, db.Columns.Prompt.Architecture, db.Columns.Prompt.Code, db.Columns.Prompt.Security, db.Columns.Prompt.Tests, db.Columns.Prompt.StatusID:
		v = db.WithSort(db.NewSortField(ops.SortColumn, ops.SortDesc))
	}

	return v
}

// Count returns count Prompts according to conditions in search params.
//
//zenrpc:search PromptSearch
//zenrpc:return int
//zenrpc:500 Internal Error
func (s PromptService) Count(ctx context.Context, search *PromptSearch) (int, error) {
	count, err := s.projectRepo.CountPrompts(ctx, search.ToDB())
	if err != nil {
		return 0, InternalError(err)
	}
	return count, nil
}

// Get returns а list of Prompts according to conditions in search params.
//
//zenrpc:search PromptSearch
//zenrpc:viewOps ViewOps
//zenrpc:return []PromptSummary
//zenrpc:500 Internal Error
func (s PromptService) Get(ctx context.Context, search *PromptSearch, viewOps *ViewOps) ([]PromptSummary, error) {
	list, err := s.projectRepo.PromptsByFilters(ctx, search.ToDB(), viewOps.Pager(), s.dbSort(viewOps), s.projectRepo.FullPrompt())
	if err != nil {
		return nil, InternalError(err)
	}
	prompts := make([]PromptSummary, 0, len(list))
	for i := range list {
		if prompt := NewPromptSummary(&list[i]); prompt != nil {
			prompts = append(prompts, *prompt)
		}
	}
	return prompts, nil
}

// GetByID returns a Prompt by its ID.
//
//zenrpc:id int
//zenrpc:return Prompt
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s PromptService) GetByID(ctx context.Context, id int) (*Prompt, error) {
	db, err := s.byID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewPrompt(db), nil
}

func (s PromptService) byID(ctx context.Context, id int) (*db.Prompt, error) {
	db, err := s.projectRepo.PromptByID(ctx, id, s.projectRepo.FullPrompt())
	if err != nil {
		return nil, InternalError(err)
	} else if db == nil {
		return nil, ErrNotFound
	}
	return db, nil
}

// Add adds a Prompt from the query.
//
//zenrpc:prompt Prompt
//zenrpc:return Prompt
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
func (s PromptService) Add(ctx context.Context, prompt Prompt) (*Prompt, error) {
	if ve := s.isValid(ctx, prompt, false); ve.HasErrors() {
		return nil, ve.Error()
	}

	db, err := s.projectRepo.AddPrompt(ctx, prompt.ToDB())
	if err != nil {
		return nil, InternalError(err)
	}
	return NewPrompt(db), nil
}

// Update updates the Prompt data identified by id from the query.
//
//zenrpc:prompts Prompt
//zenrpc:return Prompt
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s PromptService) Update(ctx context.Context, prompt Prompt) (bool, error) {
	if _, err := s.byID(ctx, prompt.ID); err != nil {
		return false, err
	}

	if ve := s.isValid(ctx, prompt, true); ve.HasErrors() {
		return false, ve.Error()
	}

	ok, err := s.projectRepo.UpdatePrompt(ctx, prompt.ToDB())
	if err != nil {
		return false, InternalError(err)
	}
	return ok, nil
}

// Delete deletes the Prompt by its ID.
//
//zenrpc:id int
//zenrpc:return isDeleted
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s PromptService) Delete(ctx context.Context, id int) (bool, error) {
	if _, err := s.byID(ctx, id); err != nil {
		return false, err
	}

	ok, err := s.projectRepo.DeletePrompt(ctx, id)
	if err != nil {
		return false, InternalError(err)
	}
	return ok, err
}

// Validate verifies that Prompt data is valid.
//
//zenrpc:prompt Prompt
//zenrpc:return []FieldError
//zenrpc:500 Internal Error
func (s PromptService) Validate(ctx context.Context, prompt Prompt) ([]FieldError, error) {
	isUpdate := prompt.ID != 0
	if isUpdate {
		_, err := s.byID(ctx, prompt.ID)
		if err != nil {
			return nil, err
		}
	}

	ve := s.isValid(ctx, prompt, isUpdate)
	if ve.HasInternalError() {
		return nil, ve.Error()
	}

	return ve.Fields(), nil
}

func (s PromptService) isValid(ctx context.Context, prompt Prompt, isUpdate bool) Validator {
	_ = isUpdate

	var v Validator

	if v.CheckBasic(ctx, prompt); v.HasInternalError() {
		return v
	}

	// custom validation starts here
	return v
}

type SlackChannelService struct {
	zenrpc.Service
	embedlog.Logger
	projectRepo db.ProjectRepo
}

func NewSlackChannelService(dbo db.DB, logger embedlog.Logger) *SlackChannelService {
	return &SlackChannelService{
		Logger:      logger,
		projectRepo: db.NewProjectRepo(dbo),
	}
}

func (s SlackChannelService) dbSort(ops *ViewOps) db.OpFunc {
	v := s.projectRepo.DefaultSlackChannelSort()
	if ops == nil {
		return v
	}

	switch ops.SortColumn {
	case db.Columns.SlackChannel.ID, db.Columns.SlackChannel.Title, db.Columns.SlackChannel.Channel, db.Columns.SlackChannel.WebhookURL, db.Columns.SlackChannel.StatusID:
		v = db.WithSort(db.NewSortField(ops.SortColumn, ops.SortDesc))
	}

	return v
}

// Count returns count SlackChannels according to conditions in search params.
//
//zenrpc:search SlackChannelSearch
//zenrpc:return int
//zenrpc:500 Internal Error
func (s SlackChannelService) Count(ctx context.Context, search *SlackChannelSearch) (int, error) {
	count, err := s.projectRepo.CountSlackChannels(ctx, search.ToDB())
	if err != nil {
		return 0, InternalError(err)
	}
	return count, nil
}

// Get returns а list of SlackChannels according to conditions in search params.
//
//zenrpc:search SlackChannelSearch
//zenrpc:viewOps ViewOps
//zenrpc:return []SlackChannelSummary
//zenrpc:500 Internal Error
func (s SlackChannelService) Get(ctx context.Context, search *SlackChannelSearch, viewOps *ViewOps) ([]SlackChannelSummary, error) {
	list, err := s.projectRepo.SlackChannelsByFilters(ctx, search.ToDB(), viewOps.Pager(), s.dbSort(viewOps), s.projectRepo.FullSlackChannel())
	if err != nil {
		return nil, InternalError(err)
	}
	slackChannels := make([]SlackChannelSummary, 0, len(list))
	for i := range list {
		if slackChannel := NewSlackChannelSummary(&list[i]); slackChannel != nil {
			slackChannels = append(slackChannels, *slackChannel)
		}
	}
	return slackChannels, nil
}

// GetByID returns a SlackChannel by its ID.
//
//zenrpc:id int
//zenrpc:return SlackChannel
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s SlackChannelService) GetByID(ctx context.Context, id int) (*SlackChannel, error) {
	db, err := s.byID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewSlackChannel(db), nil
}

func (s SlackChannelService) byID(ctx context.Context, id int) (*db.SlackChannel, error) {
	db, err := s.projectRepo.SlackChannelByID(ctx, id, s.projectRepo.FullSlackChannel())
	if err != nil {
		return nil, InternalError(err)
	} else if db == nil {
		return nil, ErrNotFound
	}
	return db, nil
}

// Add adds a SlackChannel from the query.
//
//zenrpc:slackChannel SlackChannel
//zenrpc:return SlackChannel
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
func (s SlackChannelService) Add(ctx context.Context, slackChannel SlackChannel) (*SlackChannel, error) {
	if ve := s.isValid(ctx, slackChannel, false); ve.HasErrors() {
		return nil, ve.Error()
	}

	db, err := s.projectRepo.AddSlackChannel(ctx, slackChannel.ToDB())
	if err != nil {
		return nil, InternalError(err)
	}
	return NewSlackChannel(db), nil
}

// Update updates the SlackChannel data identified by id from the query.
//
//zenrpc:slackChannels SlackChannel
//zenrpc:return SlackChannel
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s SlackChannelService) Update(ctx context.Context, slackChannel SlackChannel) (bool, error) {
	if _, err := s.byID(ctx, slackChannel.ID); err != nil {
		return false, err
	}

	if ve := s.isValid(ctx, slackChannel, true); ve.HasErrors() {
		return false, ve.Error()
	}

	ok, err := s.projectRepo.UpdateSlackChannel(ctx, slackChannel.ToDB())
	if err != nil {
		return false, InternalError(err)
	}
	return ok, nil
}

// Delete deletes the SlackChannel by its ID.
//
//zenrpc:id int
//zenrpc:return isDeleted
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s SlackChannelService) Delete(ctx context.Context, id int) (bool, error) {
	if _, err := s.byID(ctx, id); err != nil {
		return false, err
	}

	ok, err := s.projectRepo.DeleteSlackChannel(ctx, id)
	if err != nil {
		return false, InternalError(err)
	}
	return ok, err
}

// Validate verifies that SlackChannel data is valid.
//
//zenrpc:slackChannel SlackChannel
//zenrpc:return []FieldError
//zenrpc:500 Internal Error
func (s SlackChannelService) Validate(ctx context.Context, slackChannel SlackChannel) ([]FieldError, error) {
	isUpdate := slackChannel.ID != 0
	if isUpdate {
		_, err := s.byID(ctx, slackChannel.ID)
		if err != nil {
			return nil, err
		}
	}

	ve := s.isValid(ctx, slackChannel, isUpdate)
	if ve.HasInternalError() {
		return nil, ve.Error()
	}

	return ve.Fields(), nil
}

func (s SlackChannelService) isValid(ctx context.Context, slackChannel SlackChannel, isUpdate bool) Validator {
	_ = isUpdate
	var v Validator

	if v.CheckBasic(ctx, slackChannel); v.HasInternalError() {
		return v
	}

	// custom validation starts here
	return v
}

type TaskTrackerService struct {
	zenrpc.Service
	embedlog.Logger
	projectRepo db.ProjectRepo
}

func NewTaskTrackerService(dbo db.DB, logger embedlog.Logger) *TaskTrackerService {
	return &TaskTrackerService{
		Logger:      logger,
		projectRepo: db.NewProjectRepo(dbo),
	}
}

func (s TaskTrackerService) dbSort(ops *ViewOps) db.OpFunc {
	v := s.projectRepo.DefaultTaskTrackerSort()
	if ops == nil {
		return v
	}

	switch ops.SortColumn {
	case db.Columns.TaskTracker.ID, db.Columns.TaskTracker.Title, db.Columns.TaskTracker.URL, db.Columns.TaskTracker.AuthToken, db.Columns.TaskTracker.FetchPrompt, db.Columns.TaskTracker.StatusID:
		v = db.WithSort(db.NewSortField(ops.SortColumn, ops.SortDesc))
	}

	return v
}

// Count returns count TaskTrackers according to conditions in search params.
//
//zenrpc:search TaskTrackerSearch
//zenrpc:return int
//zenrpc:500 Internal Error
func (s TaskTrackerService) Count(ctx context.Context, search *TaskTrackerSearch) (int, error) {
	count, err := s.projectRepo.CountTaskTrackers(ctx, search.ToDB())
	if err != nil {
		return 0, InternalError(err)
	}
	return count, nil
}

// Get returns а list of TaskTrackers according to conditions in search params.
//
//zenrpc:search TaskTrackerSearch
//zenrpc:viewOps ViewOps
//zenrpc:return []TaskTrackerSummary
//zenrpc:500 Internal Error
func (s TaskTrackerService) Get(ctx context.Context, search *TaskTrackerSearch, viewOps *ViewOps) ([]TaskTrackerSummary, error) {
	list, err := s.projectRepo.TaskTrackersByFilters(ctx, search.ToDB(), viewOps.Pager(), s.dbSort(viewOps), s.projectRepo.FullTaskTracker())
	if err != nil {
		return nil, InternalError(err)
	}
	taskTrackers := make([]TaskTrackerSummary, 0, len(list))
	for i := range list {
		if taskTracker := NewTaskTrackerSummary(&list[i]); taskTracker != nil {
			taskTrackers = append(taskTrackers, *taskTracker)
		}
	}
	return taskTrackers, nil
}

// GetByID returns a TaskTracker by its ID.
//
//zenrpc:id int
//zenrpc:return TaskTracker
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s TaskTrackerService) GetByID(ctx context.Context, id int) (*TaskTracker, error) {
	db, err := s.byID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewTaskTracker(db), nil
}

func (s TaskTrackerService) byID(ctx context.Context, id int) (*db.TaskTracker, error) {
	db, err := s.projectRepo.TaskTrackerByID(ctx, id, s.projectRepo.FullTaskTracker())
	if err != nil {
		return nil, InternalError(err)
	} else if db == nil {
		return nil, ErrNotFound
	}
	return db, nil
}

// Add adds a TaskTracker from the query.
//
//zenrpc:taskTracker TaskTracker
//zenrpc:return TaskTracker
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
func (s TaskTrackerService) Add(ctx context.Context, taskTracker TaskTracker) (*TaskTracker, error) {
	if ve := s.isValid(ctx, taskTracker, false); ve.HasErrors() {
		return nil, ve.Error()
	}

	db, err := s.projectRepo.AddTaskTracker(ctx, taskTracker.ToDB())
	if err != nil {
		return nil, InternalError(err)
	}
	return NewTaskTracker(db), nil
}

// Update updates the TaskTracker data identified by id from the query.
//
//zenrpc:taskTrackers TaskTracker
//zenrpc:return TaskTracker
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s TaskTrackerService) Update(ctx context.Context, taskTracker TaskTracker) (bool, error) {
	if _, err := s.byID(ctx, taskTracker.ID); err != nil {
		return false, err
	}

	if ve := s.isValid(ctx, taskTracker, true); ve.HasErrors() {
		return false, ve.Error()
	}

	ok, err := s.projectRepo.UpdateTaskTracker(ctx, taskTracker.ToDB())
	if err != nil {
		return false, InternalError(err)
	}
	return ok, nil
}

// Delete deletes the TaskTracker by its ID.
//
//zenrpc:id int
//zenrpc:return isDeleted
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s TaskTrackerService) Delete(ctx context.Context, id int) (bool, error) {
	if _, err := s.byID(ctx, id); err != nil {
		return false, err
	}

	ok, err := s.projectRepo.DeleteTaskTracker(ctx, id)
	if err != nil {
		return false, InternalError(err)
	}
	return ok, err
}

// Validate verifies that TaskTracker data is valid.
//
//zenrpc:taskTracker TaskTracker
//zenrpc:return []FieldError
//zenrpc:500 Internal Error
func (s TaskTrackerService) Validate(ctx context.Context, taskTracker TaskTracker) ([]FieldError, error) {
	isUpdate := taskTracker.ID != 0
	if isUpdate {
		_, err := s.byID(ctx, taskTracker.ID)
		if err != nil {
			return nil, err
		}
	}

	ve := s.isValid(ctx, taskTracker, isUpdate)
	if ve.HasInternalError() {
		return nil, ve.Error()
	}

	return ve.Fields(), nil
}

func (s TaskTrackerService) isValid(ctx context.Context, taskTracker TaskTracker, isUpdate bool) Validator {
	_ = isUpdate

	var v Validator

	if v.CheckBasic(ctx, taskTracker); v.HasInternalError() {
		return v
	}

	// custom validation starts here
	return v
}
