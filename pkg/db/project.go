package db

import (
	"context"
	"errors"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type ProjectRepo struct {
	db      orm.DB
	filters map[string][]Filter
	sort    map[string][]SortField
	join    map[string][]string
}

// NewProjectRepo returns new repository
func NewProjectRepo(db orm.DB) ProjectRepo {
	return ProjectRepo{
		db: db,
		filters: map[string][]Filter{
			Tables.Project.Name:      {StatusFilter},
			Tables.Prompt.Name:       {StatusFilter},
			Tables.SlackChannel.Name: {StatusFilter},
			Tables.TaskTracker.Name:  {StatusFilter},
		},
		sort: map[string][]SortField{
			Tables.Project.Name:      {{Column: Columns.Project.CreatedAt, Direction: SortDesc}},
			Tables.Prompt.Name:       {{Column: Columns.Prompt.CreatedAt, Direction: SortDesc}},
			Tables.SlackChannel.Name: {{Column: Columns.SlackChannel.Title, Direction: SortAsc}},
			Tables.TaskTracker.Name:  {{Column: Columns.TaskTracker.CreatedAt, Direction: SortDesc}},
		},
		join: map[string][]string{
			Tables.Project.Name:      {TableColumns, Columns.Project.Prompt, Columns.Project.TaskTracker, Columns.Project.SlackChannel},
			Tables.Prompt.Name:       {TableColumns},
			Tables.SlackChannel.Name: {TableColumns},
			Tables.TaskTracker.Name:  {TableColumns},
		},
	}
}

// WithTransaction is a function that wraps ProjectRepo with pg.Tx transaction.
func (pr ProjectRepo) WithTransaction(tx *pg.Tx) ProjectRepo {
	pr.db = tx
	return pr
}

// WithEnabledOnly is a function that adds "statusId"=1 as base filter.
func (pr ProjectRepo) WithEnabledOnly() ProjectRepo {
	f := make(map[string][]Filter, len(pr.filters))
	for i := range pr.filters {
		f[i] = make([]Filter, len(pr.filters[i]))
		copy(f[i], pr.filters[i])
		f[i] = append(f[i], StatusEnabledFilter)
	}
	pr.filters = f

	return pr
}

/*** Project ***/

// FullProject returns full joins with all columns
func (pr ProjectRepo) FullProject() OpFunc {
	return WithColumns(pr.join[Tables.Project.Name]...)
}

// DefaultProjectSort returns default sort.
func (pr ProjectRepo) DefaultProjectSort() OpFunc {
	return WithSort(pr.sort[Tables.Project.Name]...)
}

// ProjectByID is a function that returns Project by ID(s) or nil.
func (pr ProjectRepo) ProjectByID(ctx context.Context, id int, ops ...OpFunc) (*Project, error) {
	return pr.OneProject(ctx, &ProjectSearch{ID: &id}, ops...)
}

// OneProject is a function that returns one Project by filters. It could return pg.ErrMultiRows.
func (pr ProjectRepo) OneProject(ctx context.Context, search *ProjectSearch, ops ...OpFunc) (*Project, error) {
	obj := &Project{}
	err := buildQuery(ctx, pr.db, obj, search, pr.filters[Tables.Project.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// ProjectsByFilters returns Project list.
func (pr ProjectRepo) ProjectsByFilters(ctx context.Context, search *ProjectSearch, pager Pager, ops ...OpFunc) (projects []Project, err error) {
	err = buildQuery(ctx, pr.db, &projects, search, pr.filters[Tables.Project.Name], pager, ops...).Select()
	return
}

// CountProjects returns count
func (pr ProjectRepo) CountProjects(ctx context.Context, search *ProjectSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, pr.db, &Project{}, search, pr.filters[Tables.Project.Name], PagerOne, ops...).Count()
}

// AddProject adds Project to DB.
func (pr ProjectRepo) AddProject(ctx context.Context, project *Project, ops ...OpFunc) (*Project, error) {
	q := pr.db.ModelContext(ctx, project)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Project.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return project, err
}

// UpdateProject updates Project in DB.
func (pr ProjectRepo) UpdateProject(ctx context.Context, project *Project, ops ...OpFunc) (bool, error) {
	q := pr.db.ModelContext(ctx, project).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Project.ID, Columns.Project.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteProject set statusId to deleted in DB.
func (pr ProjectRepo) DeleteProject(ctx context.Context, id int) (deleted bool, err error) {
	project := &Project{ID: id, StatusID: StatusDeleted}

	return pr.UpdateProject(ctx, project, WithColumns(Columns.Project.StatusID))
}

/*** Prompt ***/

// FullPrompt returns full joins with all columns
func (pr ProjectRepo) FullPrompt() OpFunc {
	return WithColumns(pr.join[Tables.Prompt.Name]...)
}

// DefaultPromptSort returns default sort.
func (pr ProjectRepo) DefaultPromptSort() OpFunc {
	return WithSort(pr.sort[Tables.Prompt.Name]...)
}

// PromptByID is a function that returns Prompt by ID(s) or nil.
func (pr ProjectRepo) PromptByID(ctx context.Context, id int, ops ...OpFunc) (*Prompt, error) {
	return pr.OnePrompt(ctx, &PromptSearch{ID: &id}, ops...)
}

// OnePrompt is a function that returns one Prompt by filters. It could return pg.ErrMultiRows.
func (pr ProjectRepo) OnePrompt(ctx context.Context, search *PromptSearch, ops ...OpFunc) (*Prompt, error) {
	obj := &Prompt{}
	err := buildQuery(ctx, pr.db, obj, search, pr.filters[Tables.Prompt.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// PromptsByFilters returns Prompt list.
func (pr ProjectRepo) PromptsByFilters(ctx context.Context, search *PromptSearch, pager Pager, ops ...OpFunc) (prompts []Prompt, err error) {
	err = buildQuery(ctx, pr.db, &prompts, search, pr.filters[Tables.Prompt.Name], pager, ops...).Select()
	return
}

// CountPrompts returns count
func (pr ProjectRepo) CountPrompts(ctx context.Context, search *PromptSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, pr.db, &Prompt{}, search, pr.filters[Tables.Prompt.Name], PagerOne, ops...).Count()
}

// AddPrompt adds Prompt to DB.
func (pr ProjectRepo) AddPrompt(ctx context.Context, prompt *Prompt, ops ...OpFunc) (*Prompt, error) {
	q := pr.db.ModelContext(ctx, prompt)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Prompt.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return prompt, err
}

// UpdatePrompt updates Prompt in DB.
func (pr ProjectRepo) UpdatePrompt(ctx context.Context, prompt *Prompt, ops ...OpFunc) (bool, error) {
	q := pr.db.ModelContext(ctx, prompt).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Prompt.ID, Columns.Prompt.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeletePrompt set statusId to deleted in DB.
func (pr ProjectRepo) DeletePrompt(ctx context.Context, id int) (deleted bool, err error) {
	prompt := &Prompt{ID: id, StatusID: StatusDeleted}

	return pr.UpdatePrompt(ctx, prompt, WithColumns(Columns.Prompt.StatusID))
}

/*** SlackChannel ***/

// FullSlackChannel returns full joins with all columns
func (pr ProjectRepo) FullSlackChannel() OpFunc {
	return WithColumns(pr.join[Tables.SlackChannel.Name]...)
}

// DefaultSlackChannelSort returns default sort.
func (pr ProjectRepo) DefaultSlackChannelSort() OpFunc {
	return WithSort(pr.sort[Tables.SlackChannel.Name]...)
}

// SlackChannelByID is a function that returns SlackChannel by ID(s) or nil.
func (pr ProjectRepo) SlackChannelByID(ctx context.Context, id int, ops ...OpFunc) (*SlackChannel, error) {
	return pr.OneSlackChannel(ctx, &SlackChannelSearch{ID: &id}, ops...)
}

// OneSlackChannel is a function that returns one SlackChannel by filters. It could return pg.ErrMultiRows.
func (pr ProjectRepo) OneSlackChannel(ctx context.Context, search *SlackChannelSearch, ops ...OpFunc) (*SlackChannel, error) {
	obj := &SlackChannel{}
	err := buildQuery(ctx, pr.db, obj, search, pr.filters[Tables.SlackChannel.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// SlackChannelsByFilters returns SlackChannel list.
func (pr ProjectRepo) SlackChannelsByFilters(ctx context.Context, search *SlackChannelSearch, pager Pager, ops ...OpFunc) (slackChannels []SlackChannel, err error) {
	err = buildQuery(ctx, pr.db, &slackChannels, search, pr.filters[Tables.SlackChannel.Name], pager, ops...).Select()
	return
}

// CountSlackChannels returns count
func (pr ProjectRepo) CountSlackChannels(ctx context.Context, search *SlackChannelSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, pr.db, &SlackChannel{}, search, pr.filters[Tables.SlackChannel.Name], PagerOne, ops...).Count()
}

// AddSlackChannel adds SlackChannel to DB.
func (pr ProjectRepo) AddSlackChannel(ctx context.Context, slackChannel *SlackChannel, ops ...OpFunc) (*SlackChannel, error) {
	q := pr.db.ModelContext(ctx, slackChannel)
	applyOps(q, ops...)
	_, err := q.Insert()

	return slackChannel, err
}

// UpdateSlackChannel updates SlackChannel in DB.
func (pr ProjectRepo) UpdateSlackChannel(ctx context.Context, slackChannel *SlackChannel, ops ...OpFunc) (bool, error) {
	q := pr.db.ModelContext(ctx, slackChannel).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.SlackChannel.ID)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteSlackChannel set statusId to deleted in DB.
func (pr ProjectRepo) DeleteSlackChannel(ctx context.Context, id int) (deleted bool, err error) {
	slackChannel := &SlackChannel{ID: id, StatusID: StatusDeleted}

	return pr.UpdateSlackChannel(ctx, slackChannel, WithColumns(Columns.SlackChannel.StatusID))
}

/*** TaskTracker ***/

// FullTaskTracker returns full joins with all columns
func (pr ProjectRepo) FullTaskTracker() OpFunc {
	return WithColumns(pr.join[Tables.TaskTracker.Name]...)
}

// DefaultTaskTrackerSort returns default sort.
func (pr ProjectRepo) DefaultTaskTrackerSort() OpFunc {
	return WithSort(pr.sort[Tables.TaskTracker.Name]...)
}

// TaskTrackerByID is a function that returns TaskTracker by ID(s) or nil.
func (pr ProjectRepo) TaskTrackerByID(ctx context.Context, id int, ops ...OpFunc) (*TaskTracker, error) {
	return pr.OneTaskTracker(ctx, &TaskTrackerSearch{ID: &id}, ops...)
}

// OneTaskTracker is a function that returns one TaskTracker by filters. It could return pg.ErrMultiRows.
func (pr ProjectRepo) OneTaskTracker(ctx context.Context, search *TaskTrackerSearch, ops ...OpFunc) (*TaskTracker, error) {
	obj := &TaskTracker{}
	err := buildQuery(ctx, pr.db, obj, search, pr.filters[Tables.TaskTracker.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// TaskTrackersByFilters returns TaskTracker list.
func (pr ProjectRepo) TaskTrackersByFilters(ctx context.Context, search *TaskTrackerSearch, pager Pager, ops ...OpFunc) (taskTrackers []TaskTracker, err error) {
	err = buildQuery(ctx, pr.db, &taskTrackers, search, pr.filters[Tables.TaskTracker.Name], pager, ops...).Select()
	return
}

// CountTaskTrackers returns count
func (pr ProjectRepo) CountTaskTrackers(ctx context.Context, search *TaskTrackerSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, pr.db, &TaskTracker{}, search, pr.filters[Tables.TaskTracker.Name], PagerOne, ops...).Count()
}

// AddTaskTracker adds TaskTracker to DB.
func (pr ProjectRepo) AddTaskTracker(ctx context.Context, taskTracker *TaskTracker, ops ...OpFunc) (*TaskTracker, error) {
	q := pr.db.ModelContext(ctx, taskTracker)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.TaskTracker.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return taskTracker, err
}

// UpdateTaskTracker updates TaskTracker in DB.
func (pr ProjectRepo) UpdateTaskTracker(ctx context.Context, taskTracker *TaskTracker, ops ...OpFunc) (bool, error) {
	q := pr.db.ModelContext(ctx, taskTracker).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.TaskTracker.ID, Columns.TaskTracker.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteTaskTracker set statusId to deleted in DB.
func (pr ProjectRepo) DeleteTaskTracker(ctx context.Context, id int) (deleted bool, err error) {
	taskTracker := &TaskTracker{ID: id, StatusID: StatusDeleted}

	return pr.UpdateTaskTracker(ctx, taskTracker, WithColumns(Columns.TaskTracker.StatusID))
}
