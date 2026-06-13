//nolint:dupl
package vt

import (
	"reviewsrv/pkg/db"
)

type RunnerProfile struct {
	ID          int                    `json:"id"`
	Title       string                 `json:"title" validate:"required,max=255"`
	Runner      string                 `json:"runner" validate:"required,max=32"`
	Model       *string                `json:"model" validate:"omitempty,max=128"`
	Effort      *string                `json:"effort" validate:"omitempty,max=16"`
	APIProvider *string                `json:"apiProvider" validate:"omitempty,max=32"`
	APIBaseURL  *string                `json:"apiBaseURL" validate:"omitempty,max=255"`
	Token       *string                `json:"token,omitempty" validate:"omitempty,max=255"` // write-only: nil on read, set-or-keep on write
	TokenMasked string                 `json:"tokenMasked"`                                  // read-only masked display
	HasToken    bool                   `json:"hasToken"`                                     // read-only
	Params      db.RunnerProfileParams `json:"params"`
	IsDefault   bool                   `json:"isDefault"`
	StatusID    int                    `json:"statusId" validate:"required,status"`

	Status *Status `json:"status"`
}

func (rp *RunnerProfile) ToDB() *db.RunnerProfile {
	if rp == nil {
		return nil
	}

	return &db.RunnerProfile{
		ID:          rp.ID,
		Title:       rp.Title,
		Runner:      rp.Runner,
		Model:       rp.Model,
		Effort:      rp.Effort,
		APIProvider: rp.APIProvider,
		APIBaseURL:  rp.APIBaseURL,
		Token:       rp.Token,
		Params:      rp.Params,
		IsDefault:   rp.IsDefault,
		StatusID:    rp.StatusID,
	}
}

type RunnerProfileSearch struct {
	ID       *int    `json:"id"`
	Title    *string `json:"title"`
	Runner   *string `json:"runner"`
	StatusID *int    `json:"statusId"`
	IDs      []int   `json:"ids"`
}

func (rps *RunnerProfileSearch) ToDB() *db.RunnerProfileSearch {
	if rps == nil {
		return nil
	}

	return &db.RunnerProfileSearch{
		ID:          rps.ID,
		TitleILike:  rps.Title,
		RunnerILike: rps.Runner,
		StatusID:    rps.StatusID,
		IDs:         rps.IDs,
	}
}

type RunnerProfileSummary struct {
	ID        int     `json:"id"`
	Title     string  `json:"title"`
	Runner    string  `json:"runner"`
	Model     *string `json:"model"`
	Effort    *string `json:"effort"`
	IsDefault bool    `json:"isDefault"`

	Status *Status `json:"status"`
}
