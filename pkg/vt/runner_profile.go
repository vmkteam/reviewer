package vt

import (
	"context"

	"reviewsrv/pkg/db"

	"github.com/vmkteam/embedlog"
	"github.com/vmkteam/zenrpc/v2"
)

// Allowed enum values for a runner profile. Kept in sync with pkg/reviewer/runner
// and pkg/reviewer/direct; validated here so the admin can't save an unusable profile.
var (
	validRunners   = map[string]bool{"claude": true, "opencode": true, "codex": true, "direct": true}
	validProviders = map[string]bool{"anthropic": true, "deepseek": true, "openai-compat": true}
	//nolint:goconst // "max" here is an effort level, not the FieldErrorMax error code
	validEfforts = map[string]bool{"low": true, "medium": true, "high": true, "xhigh": true, "max": true}
)

type RunnerProfileService struct {
	zenrpc.Service
	embedlog.Logger
	projectRepo db.ProjectRepo
}

func NewRunnerProfileService(dbo db.DB, logger embedlog.Logger) *RunnerProfileService {
	return &RunnerProfileService{
		Logger:      logger,
		projectRepo: db.NewProjectRepo(dbo),
	}
}

func (s RunnerProfileService) dbSort(ops *ViewOps) db.OpFunc {
	v := s.projectRepo.DefaultRunnerProfileSort()
	if ops == nil {
		return v
	}

	switch ops.SortColumn {
	case db.Columns.RunnerProfile.ID, db.Columns.RunnerProfile.Title, db.Columns.RunnerProfile.Runner, db.Columns.RunnerProfile.IsDefault, db.Columns.RunnerProfile.StatusID:
		v = db.WithSort(db.NewSortField(ops.SortColumn, ops.SortDesc))
	}

	return v
}

// Count returns count RunnerProfiles according to conditions in search params.
//
//zenrpc:search RunnerProfileSearch
//zenrpc:return int
//zenrpc:500 Internal Error
func (s RunnerProfileService) Count(ctx context.Context, search *RunnerProfileSearch) (int, error) {
	count, err := s.projectRepo.CountRunnerProfiles(ctx, search.ToDB())
	if err != nil {
		return 0, InternalError(err)
	}
	return count, nil
}

// Get returns а list of RunnerProfiles according to conditions in search params.
//
//zenrpc:search RunnerProfileSearch
//zenrpc:viewOps ViewOps
//zenrpc:return []RunnerProfileSummary
//zenrpc:500 Internal Error
func (s RunnerProfileService) Get(ctx context.Context, search *RunnerProfileSearch, viewOps *ViewOps) ([]RunnerProfileSummary, error) {
	list, err := s.projectRepo.RunnerProfilesByFilters(ctx, search.ToDB(), viewOps.Pager(), s.dbSort(viewOps), s.projectRepo.FullRunnerProfile())
	if err != nil {
		return nil, InternalError(err)
	}
	runnerProfiles := make([]RunnerProfileSummary, 0, len(list))
	for i := range list {
		if runnerProfile := NewRunnerProfileSummary(&list[i]); runnerProfile != nil {
			runnerProfiles = append(runnerProfiles, *runnerProfile)
		}
	}
	return runnerProfiles, nil
}

// GetByID returns a RunnerProfile by its ID.
//
//zenrpc:id int
//zenrpc:return RunnerProfile
//zenrpc:500 Internal Error
//zenrpc:404 Not Found
func (s RunnerProfileService) GetByID(ctx context.Context, id int) (*RunnerProfile, error) {
	dbProfile, err := s.byID(ctx, id)
	if err != nil {
		return nil, err
	}
	return NewRunnerProfile(dbProfile), nil
}

func (s RunnerProfileService) byID(ctx context.Context, id int) (*db.RunnerProfile, error) {
	dbProfile, err := s.projectRepo.RunnerProfileByID(ctx, id, s.projectRepo.FullRunnerProfile())
	if err != nil {
		return nil, InternalError(err)
	} else if dbProfile == nil {
		return nil, ErrNotFound
	}
	return dbProfile, nil
}

// Add adds a RunnerProfile from the query.
//
//zenrpc:runnerProfile RunnerProfile
//zenrpc:return RunnerProfile
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
func (s RunnerProfileService) Add(ctx context.Context, runnerProfile RunnerProfile) (*RunnerProfile, error) {
	if ve := s.isValid(ctx, runnerProfile, false); ve.HasErrors() {
		return nil, ve.Error()
	}

	rp := runnerProfile.ToDB()
	if err := s.unsetCurrentDefault(ctx, rp); err != nil {
		return nil, InternalError(err)
	}

	dbProfile, err := s.projectRepo.AddRunnerProfile(ctx, rp)
	if err != nil {
		return nil, InternalError(err)
	}
	return NewRunnerProfile(dbProfile), nil
}

// Update updates the RunnerProfile data identified by id from the query.
//
//zenrpc:runnerProfiles RunnerProfile
//zenrpc:return RunnerProfile
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s RunnerProfileService) Update(ctx context.Context, runnerProfile RunnerProfile) (bool, error) {
	existing, err := s.byID(ctx, runnerProfile.ID)
	if err != nil {
		return false, err
	}

	if ve := s.isValid(ctx, runnerProfile, true); ve.HasErrors() {
		return false, ve.Error()
	}

	rp := runnerProfile.ToDB()
	// Token is write-only with set-or-keep semantics: a nil token in the request
	// means "leave the stored token unchanged" (the admin API never returns it).
	if runnerProfile.Token == nil {
		rp.Token = existing.Token
	}

	if err = s.unsetCurrentDefault(ctx, rp); err != nil {
		return false, InternalError(err)
	}

	ok, err := s.projectRepo.UpdateRunnerProfile(ctx, rp)
	if err != nil {
		return false, InternalError(err)
	}
	return ok, nil
}

// Delete deletes the RunnerProfile by its ID.
//
//zenrpc:id int
//zenrpc:return isDeleted
//zenrpc:500 Internal Error
//zenrpc:400 Validation Error
//zenrpc:404 Not Found
func (s RunnerProfileService) Delete(ctx context.Context, id int) (bool, error) {
	if _, err := s.byID(ctx, id); err != nil {
		return false, err
	}

	ok, err := s.projectRepo.DeleteRunnerProfile(ctx, id)
	if err != nil {
		return false, InternalError(err)
	}
	return ok, err
}

// Validate verifies that RunnerProfile data is valid.
//
//zenrpc:runnerProfile RunnerProfile
//zenrpc:return []FieldError
//zenrpc:500 Internal Error
func (s RunnerProfileService) Validate(ctx context.Context, runnerProfile RunnerProfile) ([]FieldError, error) {
	isUpdate := runnerProfile.ID != 0
	if isUpdate {
		_, err := s.byID(ctx, runnerProfile.ID)
		if err != nil {
			return nil, err
		}
	}

	ve := s.isValid(ctx, runnerProfile, isUpdate)
	if ve.HasInternalError() {
		return nil, ve.Error()
	}

	return ve.Fields(), nil
}

func (s RunnerProfileService) isValid(ctx context.Context, runnerProfile RunnerProfile, isUpdate bool) Validator {
	_ = isUpdate

	var v Validator

	if v.CheckBasic(ctx, runnerProfile); v.HasInternalError() {
		return v
	}

	// custom validation starts here
	if runnerProfile.Runner != "" && !validRunners[runnerProfile.Runner] {
		v.Append("runner", FieldErrorIncorrect)
	}
	if runnerProfile.APIProvider != nil && *runnerProfile.APIProvider != "" && !validProviders[*runnerProfile.APIProvider] {
		v.Append("apiProvider", FieldErrorIncorrect)
	}
	if runnerProfile.Effort != nil && *runnerProfile.Effort != "" && !validEfforts[*runnerProfile.Effort] {
		v.Append("effort", FieldErrorIncorrect)
	}

	return v
}

// unsetCurrentDefault clears isDefault on the existing default profile when a
// different profile is being marked default, so the single-default partial-unique
// index never rejects the write. No-op when rp is not default or already is it.
func (s RunnerProfileService) unsetCurrentDefault(ctx context.Context, rp *db.RunnerProfile) error {
	if !rp.IsDefault {
		return nil
	}
	wantDefault := true
	cur, err := s.projectRepo.OneRunnerProfile(ctx, &db.RunnerProfileSearch{IsDefault: &wantDefault})
	if err != nil {
		return err
	}
	if cur == nil || cur.ID == rp.ID {
		return nil
	}
	cur.IsDefault = false
	_, err = s.projectRepo.UpdateRunnerProfile(ctx, cur)
	return err
}
