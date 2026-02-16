package db

import (
	"context"
	"errors"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
)

type ReviewRepo struct {
	db      orm.DB
	filters map[string][]Filter
	sort    map[string][]SortField
	join    map[string][]string
}

// NewReviewRepo returns new repository
func NewReviewRepo(db orm.DB) ReviewRepo {
	return ReviewRepo{
		db: db,
		filters: map[string][]Filter{
			Tables.Issue.Name:      {StatusFilter},
			Tables.ReviewFile.Name: {StatusFilter},
			Tables.Review.Name:     {StatusFilter},
		},
		sort: map[string][]SortField{
			Tables.Issue.Name:      {{Column: Columns.Issue.CreatedAt, Direction: SortDesc}},
			Tables.ReviewFile.Name: {{Column: Columns.ReviewFile.CreatedAt, Direction: SortDesc}},
			Tables.Review.Name:     {{Column: Columns.Review.CreatedAt, Direction: SortDesc}},
		},
		join: map[string][]string{
			Tables.Issue.Name:      {TableColumns, Columns.Issue.ReviewFile, Columns.Issue.Review, Columns.Issue.User},
			Tables.ReviewFile.Name: {TableColumns, Columns.ReviewFile.Review},
			Tables.Review.Name:     {TableColumns, Columns.Review.Project, Columns.Review.Prompt},
		},
	}
}

// WithTransaction is a function that wraps ReviewRepo with pg.Tx transaction.
func (rr ReviewRepo) WithTransaction(tx *pg.Tx) ReviewRepo {
	rr.db = tx
	return rr
}

// WithEnabledOnly is a function that adds "statusId"=1 as base filter.
func (rr ReviewRepo) WithEnabledOnly() ReviewRepo {
	f := make(map[string][]Filter, len(rr.filters))
	for i := range rr.filters {
		f[i] = make([]Filter, len(rr.filters[i]))
		copy(f[i], rr.filters[i])
		f[i] = append(f[i], StatusEnabledFilter)
	}
	rr.filters = f

	return rr
}

/*** Issue ***/

// FullIssue returns full joins with all columns
func (rr ReviewRepo) FullIssue() OpFunc {
	return WithColumns(rr.join[Tables.Issue.Name]...)
}

// DefaultIssueSort returns default sort.
func (rr ReviewRepo) DefaultIssueSort() OpFunc {
	return WithSort(rr.sort[Tables.Issue.Name]...)
}

// IssueByID is a function that returns Issue by ID(s) or nil.
func (rr ReviewRepo) IssueByID(ctx context.Context, id int, ops ...OpFunc) (*Issue, error) {
	return rr.OneIssue(ctx, &IssueSearch{ID: &id}, ops...)
}

// OneIssue is a function that returns one Issue by filters. It could return pg.ErrMultiRows.
func (rr ReviewRepo) OneIssue(ctx context.Context, search *IssueSearch, ops ...OpFunc) (*Issue, error) {
	obj := &Issue{}
	err := buildQuery(ctx, rr.db, obj, search, rr.filters[Tables.Issue.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// IssuesByFilters returns Issue list.
func (rr ReviewRepo) IssuesByFilters(ctx context.Context, search *IssueSearch, pager Pager, ops ...OpFunc) (issues []Issue, err error) {
	err = buildQuery(ctx, rr.db, &issues, search, rr.filters[Tables.Issue.Name], pager, ops...).Select()
	return
}

// CountIssues returns count
func (rr ReviewRepo) CountIssues(ctx context.Context, search *IssueSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, rr.db, &Issue{}, search, rr.filters[Tables.Issue.Name], PagerOne, ops...).Count()
}

// AddIssue adds Issue to DB.
func (rr ReviewRepo) AddIssue(ctx context.Context, issue *Issue, ops ...OpFunc) (*Issue, error) {
	q := rr.db.ModelContext(ctx, issue)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Issue.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return issue, err
}

// UpdateIssue updates Issue in DB.
func (rr ReviewRepo) UpdateIssue(ctx context.Context, issue *Issue, ops ...OpFunc) (bool, error) {
	q := rr.db.ModelContext(ctx, issue).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Issue.ID, Columns.Issue.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteIssue set statusId to deleted in DB.
func (rr ReviewRepo) DeleteIssue(ctx context.Context, id int) (deleted bool, err error) {
	issue := &Issue{ID: id, StatusID: StatusDeleted}

	return rr.UpdateIssue(ctx, issue, WithColumns(Columns.Issue.StatusID))
}

/*** ReviewFile ***/

// FullReviewFile returns full joins with all columns
func (rr ReviewRepo) FullReviewFile() OpFunc {
	return WithColumns(rr.join[Tables.ReviewFile.Name]...)
}

// DefaultReviewFileSort returns default sort.
func (rr ReviewRepo) DefaultReviewFileSort() OpFunc {
	return WithSort(rr.sort[Tables.ReviewFile.Name]...)
}

// ReviewFileByID is a function that returns ReviewFile by ID(s) or nil.
func (rr ReviewRepo) ReviewFileByID(ctx context.Context, id int, ops ...OpFunc) (*ReviewFile, error) {
	return rr.OneReviewFile(ctx, &ReviewFileSearch{ID: &id}, ops...)
}

// OneReviewFile is a function that returns one ReviewFile by filters. It could return pg.ErrMultiRows.
func (rr ReviewRepo) OneReviewFile(ctx context.Context, search *ReviewFileSearch, ops ...OpFunc) (*ReviewFile, error) {
	obj := &ReviewFile{}
	err := buildQuery(ctx, rr.db, obj, search, rr.filters[Tables.ReviewFile.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// ReviewFilesByFilters returns ReviewFile list.
func (rr ReviewRepo) ReviewFilesByFilters(ctx context.Context, search *ReviewFileSearch, pager Pager, ops ...OpFunc) (reviewFiles []ReviewFile, err error) {
	err = buildQuery(ctx, rr.db, &reviewFiles, search, rr.filters[Tables.ReviewFile.Name], pager, ops...).Select()
	return
}

// CountReviewFiles returns count
func (rr ReviewRepo) CountReviewFiles(ctx context.Context, search *ReviewFileSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, rr.db, &ReviewFile{}, search, rr.filters[Tables.ReviewFile.Name], PagerOne, ops...).Count()
}

// AddReviewFile adds ReviewFile to DB.
func (rr ReviewRepo) AddReviewFile(ctx context.Context, reviewFile *ReviewFile, ops ...OpFunc) (*ReviewFile, error) {
	q := rr.db.ModelContext(ctx, reviewFile)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.ReviewFile.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return reviewFile, err
}

// UpdateReviewFile updates ReviewFile in DB.
func (rr ReviewRepo) UpdateReviewFile(ctx context.Context, reviewFile *ReviewFile, ops ...OpFunc) (bool, error) {
	q := rr.db.ModelContext(ctx, reviewFile).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.ReviewFile.ID, Columns.ReviewFile.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteReviewFile set statusId to deleted in DB.
func (rr ReviewRepo) DeleteReviewFile(ctx context.Context, id int) (deleted bool, err error) {
	reviewFile := &ReviewFile{ID: id, StatusID: StatusDeleted}

	return rr.UpdateReviewFile(ctx, reviewFile, WithColumns(Columns.ReviewFile.StatusID))
}

/*** Review ***/

// FullReview returns full joins with all columns
func (rr ReviewRepo) FullReview() OpFunc {
	return WithColumns(rr.join[Tables.Review.Name]...)
}

// DefaultReviewSort returns default sort.
func (rr ReviewRepo) DefaultReviewSort() OpFunc {
	return WithSort(rr.sort[Tables.Review.Name]...)
}

// ReviewByID is a function that returns Review by ID(s) or nil.
func (rr ReviewRepo) ReviewByID(ctx context.Context, id int, ops ...OpFunc) (*Review, error) {
	return rr.OneReview(ctx, &ReviewSearch{ID: &id}, ops...)
}

// OneReview is a function that returns one Review by filters. It could return pg.ErrMultiRows.
func (rr ReviewRepo) OneReview(ctx context.Context, search *ReviewSearch, ops ...OpFunc) (*Review, error) {
	obj := &Review{}
	err := buildQuery(ctx, rr.db, obj, search, rr.filters[Tables.Review.Name], PagerTwo, ops...).Select()

	if errors.Is(err, pg.ErrMultiRows) {
		return nil, err
	} else if errors.Is(err, pg.ErrNoRows) {
		return nil, nil
	}

	return obj, err
}

// ReviewsByFilters returns Review list.
func (rr ReviewRepo) ReviewsByFilters(ctx context.Context, search *ReviewSearch, pager Pager, ops ...OpFunc) (reviews []Review, err error) {
	err = buildQuery(ctx, rr.db, &reviews, search, rr.filters[Tables.Review.Name], pager, ops...).Select()
	return
}

// CountReviews returns count
func (rr ReviewRepo) CountReviews(ctx context.Context, search *ReviewSearch, ops ...OpFunc) (int, error) {
	return buildQuery(ctx, rr.db, &Review{}, search, rr.filters[Tables.Review.Name], PagerOne, ops...).Count()
}

// AddReview adds Review to DB.
func (rr ReviewRepo) AddReview(ctx context.Context, review *Review, ops ...OpFunc) (*Review, error) {
	q := rr.db.ModelContext(ctx, review)
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Review.CreatedAt)
	}
	applyOps(q, ops...)
	_, err := q.Insert()

	return review, err
}

// UpdateReview updates Review in DB.
func (rr ReviewRepo) UpdateReview(ctx context.Context, review *Review, ops ...OpFunc) (bool, error) {
	q := rr.db.ModelContext(ctx, review).WherePK()
	if len(ops) == 0 {
		q = q.ExcludeColumn(Columns.Review.ID, Columns.Review.CreatedAt)
	}
	applyOps(q, ops...)
	res, err := q.Update()
	if err != nil {
		return false, err
	}

	return res.RowsAffected() > 0, err
}

// DeleteReview set statusId to deleted in DB.
func (rr ReviewRepo) DeleteReview(ctx context.Context, id int) (deleted bool, err error) {
	review := &Review{ID: id, StatusID: StatusDeleted}

	return rr.UpdateReview(ctx, review, WithColumns(Columns.Review.StatusID))
}
