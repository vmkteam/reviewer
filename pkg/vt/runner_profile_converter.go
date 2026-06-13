package vt

import (
	"reviewsrv/pkg/db"
)

func NewRunnerProfile(in *db.RunnerProfile) *RunnerProfile {
	if in == nil {
		return nil
	}

	return &RunnerProfile{
		ID:          in.ID,
		Title:       in.Title,
		Runner:      in.Runner,
		Model:       in.Model,
		Effort:      in.Effort,
		APIProvider: in.APIProvider,
		APIBaseURL:  in.APIBaseURL,
		Token:       nil, // never expose the raw token to the admin API
		TokenMasked: maskSecret(in.Token),
		HasToken:    in.Token != nil && *in.Token != "",
		Params:      in.Params,
		IsDefault:   in.IsDefault,
		StatusID:    in.StatusID,

		Status: NewStatus(in.StatusID),
	}
}

func NewRunnerProfileSummary(in *db.RunnerProfile) *RunnerProfileSummary {
	if in == nil {
		return nil
	}

	return &RunnerProfileSummary{
		ID:        in.ID,
		Title:     in.Title,
		Runner:    in.Runner,
		Model:     in.Model,
		Effort:    in.Effort,
		IsDefault: in.IsDefault,

		Status: NewStatus(in.StatusID),
	}
}
