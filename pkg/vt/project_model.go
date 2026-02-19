//nolint:dupl
package vt

import (
	"reviewsrv/pkg/db"
)

type Project struct {
	ID             int     `json:"id"`
	Title          string  `json:"title" validate:"required,max=255"`
	VcsURL         string  `json:"vcsURL" validate:"required,http_url,max=255"`
	Language       string  `json:"language" validate:"required,max=32"`
	ProjectKey     string  `json:"projectKey"`
	PromptID       int     `json:"promptId" validate:"required"`
	TaskTrackerID  *int    `json:"taskTrackerId"`
	SlackChannelID *int    `json:"slackChannelId"`
	StatusID       int     `json:"statusId" validate:"required,status"`
	Instructions   *string `json:"instructions"`

	Prompt       *PromptSummary       `json:"prompt"`
	TaskTracker  *TaskTrackerSummary  `json:"taskTracker"`
	SlackChannel *SlackChannelSummary `json:"slackChannel"`
	Status       *Status              `json:"status"`
}

func (p *Project) ToDB() *db.Project {
	if p == nil {
		return nil
	}

	project := &db.Project{
		ID:             p.ID,
		Title:          p.Title,
		VcsURL:         p.VcsURL,
		Language:       p.Language,
		ProjectKey:     p.ProjectKey,
		PromptID:       p.PromptID,
		TaskTrackerID:  p.TaskTrackerID,
		SlackChannelID: p.SlackChannelID,
		StatusID:       p.StatusID,
		Instructions:   p.Instructions,
	}

	return project
}

type ProjectSearch struct {
	ID             *int    `json:"id"`
	Title          *string `json:"title"`
	VcsURL         *string `json:"vcsURL"`
	Language       *string `json:"language"`
	ProjectKey     *string `json:"projectKey"`
	PromptID       *int    `json:"promptId"`
	TaskTrackerID  *int    `json:"taskTrackerId"`
	SlackChannelID *int    `json:"slackChannelId"`
	StatusID       *int    `json:"statusId"`
	IDs            []int   `json:"ids"`
}

func (ps *ProjectSearch) ToDB() *db.ProjectSearch {
	if ps == nil {
		return nil
	}

	return &db.ProjectSearch{
		ID:             ps.ID,
		TitleILike:     ps.Title,
		VcsURLILike:    ps.VcsURL,
		LanguageILike:  ps.Language,
		ProjectKey:     ps.ProjectKey,
		PromptID:       ps.PromptID,
		TaskTrackerID:  ps.TaskTrackerID,
		SlackChannelID: ps.SlackChannelID,
		StatusID:       ps.StatusID,
		IDs:            ps.IDs,
	}
}

type ProjectSummary struct {
	ID             int    `json:"id"`
	Title          string `json:"title"`
	VcsURL         string `json:"vcsURL"`
	Language       string `json:"language"`
	ProjectKey     string `json:"projectKey"`
	PromptID       int    `json:"promptId"`
	TaskTrackerID  *int   `json:"taskTrackerId"`
	SlackChannelID *int   `json:"slackChannelId"`

	Prompt       *PromptSummary       `json:"prompt"`
	TaskTracker  *TaskTrackerSummary  `json:"taskTracker"`
	SlackChannel *SlackChannelSummary `json:"slackChannel"`
	Status       *Status              `json:"status"`
}

type Prompt struct {
	ID           int    `json:"id"`
	Title        string `json:"title" validate:"required,max=255"`
	Common       string `json:"common" validate:"required"`
	Architecture string `json:"architecture" validate:"required"`
	Code         string `json:"code" validate:"required"`
	Security     string `json:"security" validate:"required"`
	Tests        string `json:"tests" validate:"required"`
	StatusID     int    `json:"statusId" validate:"required,status"`

	Status *Status `json:"status"`
}

func (p *Prompt) ToDB() *db.Prompt {
	if p == nil {
		return nil
	}

	prompt := &db.Prompt{
		ID:           p.ID,
		Title:        p.Title,
		Common:       p.Common,
		Architecture: p.Architecture,
		Code:         p.Code,
		Security:     p.Security,
		Tests:        p.Tests,
		StatusID:     p.StatusID,
	}

	return prompt
}

type PromptSearch struct {
	ID           *int    `json:"id"`
	Title        *string `json:"title"`
	Common       *string `json:"common"`
	Architecture *string `json:"architecture"`
	Code         *string `json:"code"`
	Security     *string `json:"security"`
	Tests        *string `json:"tests"`
	StatusID     *int    `json:"statusId"`
	IDs          []int   `json:"ids"`
}

func (ps *PromptSearch) ToDB() *db.PromptSearch {
	if ps == nil {
		return nil
	}

	return &db.PromptSearch{
		ID:                ps.ID,
		TitleILike:        ps.Title,
		CommonILike:       ps.Common,
		ArchitectureILike: ps.Architecture,
		CodeILike:         ps.Code,
		SecurityILike:     ps.Security,
		TestsILike:        ps.Tests,
		StatusID:          ps.StatusID,
		IDs:               ps.IDs,
	}
}

type PromptSummary struct {
	ID           int    `json:"id"`
	Title        string `json:"title"`
	Common       string `json:"common"`
	Architecture string `json:"architecture"`
	Code         string `json:"code"`
	Security     string `json:"security"`
	Tests        string `json:"tests"`

	Status *Status `json:"status"`
}

type SlackChannel struct {
	ID         int    `json:"id"`
	Title      string `json:"title" validate:"required,max=255"`
	Channel    string `json:"channel" validate:"required,max=255"`
	WebhookURL string `json:"webhookURL" validate:"required,max=1024"`
	StatusID   int    `json:"statusId" validate:"required,status"`

	Status *Status `json:"status"`
}

func (sc *SlackChannel) ToDB() *db.SlackChannel {
	if sc == nil {
		return nil
	}

	slackChannel := &db.SlackChannel{
		ID:         sc.ID,
		Title:      sc.Title,
		Channel:    sc.Channel,
		WebhookURL: sc.WebhookURL,
		StatusID:   sc.StatusID,
	}

	return slackChannel
}

type SlackChannelSearch struct {
	ID         *int    `json:"id"`
	Title      *string `json:"title"`
	Channel    *string `json:"channel"`
	WebhookURL *string `json:"webhookURL"`
	StatusID   *int    `json:"statusId"`
	IDs        []int   `json:"ids"`
}

func (scs *SlackChannelSearch) ToDB() *db.SlackChannelSearch {
	if scs == nil {
		return nil
	}

	return &db.SlackChannelSearch{
		ID:              scs.ID,
		TitleILike:      scs.Title,
		ChannelILike:    scs.Channel,
		WebhookURLILike: scs.WebhookURL,
		StatusID:        scs.StatusID,
		IDs:             scs.IDs,
	}
}

type SlackChannelSummary struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	Channel    string `json:"channel"`
	WebhookURL string `json:"webhookURL"`

	Status *Status `json:"status"`
}

type TaskTracker struct {
	ID          int     `json:"id"`
	Title       string  `json:"title" validate:"required,max=255"`
	URL         string  `json:"url" validate:"required,max=255"`
	AuthToken   *string `json:"authToken" validate:"omitempty,max=255"`
	FetchPrompt string  `json:"fetchPrompt" validate:"required"`
	StatusID    int     `json:"statusId" validate:"required,status"`

	Status *Status `json:"status"`
}

func (tt *TaskTracker) ToDB() *db.TaskTracker {
	if tt == nil {
		return nil
	}

	taskTracker := &db.TaskTracker{
		ID:          tt.ID,
		Title:       tt.Title,
		URL:         tt.URL,
		AuthToken:   tt.AuthToken,
		FetchPrompt: tt.FetchPrompt,
		StatusID:    tt.StatusID,
	}

	return taskTracker
}

type TaskTrackerSearch struct {
	ID          *int    `json:"id"`
	Title       *string `json:"title"`
	URL         *string `json:"url"`
	AuthToken   *string `json:"authToken"`
	FetchPrompt *string `json:"fetchPrompt"`
	StatusID    *int    `json:"statusId"`
	IDs         []int   `json:"ids"`
}

func (tts *TaskTrackerSearch) ToDB() *db.TaskTrackerSearch {
	if tts == nil {
		return nil
	}

	return &db.TaskTrackerSearch{
		ID:               tts.ID,
		TitleILike:       tts.Title,
		URL:              tts.URL,
		AuthTokenILike:   tts.AuthToken,
		FetchPromptILike: tts.FetchPrompt,
		StatusID:         tts.StatusID,
		IDs:              tts.IDs,
	}
}

type TaskTrackerSummary struct {
	ID          int     `json:"id"`
	Title       string  `json:"title"`
	URL         string  `json:"url"`
	AuthToken   *string `json:"authToken"`
	FetchPrompt string  `json:"fetchPrompt"`

	Status *Status `json:"status"`
}
